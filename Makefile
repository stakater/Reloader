# note: call scripts from /scripts

.PHONY: default build build-image test stop push apply deploy release release-all manifest push

OS ?= linux
ARCH ?= ???
ALL_ARCH ?= arm64 arm amd64

BUILDER_IMAGE ?=
BASE_IMAGE    ?=
BINARY ?= Reloader
DOCKER_IMAGE ?= ghcr.io/stakater/reloader

# Default value "dev"
VERSION ?= 0.0.1

# Full image reference (used for docker-build)
IMG ?= $(DOCKER_IMAGE):v$(VERSION)

REPOSITORY_GENERIC = ${DOCKER_IMAGE}:${VERSION}
REPOSITORY_ARCH = ${DOCKER_IMAGE}:v${VERSION}-${ARCH}
BUILD=

GOCMD = go
GOFLAGS ?= $(GOFLAGS:)
LDFLAGS =
GOPROXY   ?=
GOPRIVATE ?=

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
YQ ?= $(LOCALBIN)/yq

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.17
GOLANGCI_LINT_VERSION ?= v2.6.1

YQ_VERSION ?= v4.27.5
YQ_DOWNLOAD_URL = "https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(OS)_$(ARCH)"

.PHONY: yq
yq: $(YQ) ## Download YQ locally if needed
$(YQ):
	@test -d $(LOCALBIN) || mkdir -p $(LOCALBIN)
	@curl --retry 3 -fsSL $(YQ_DOWNLOAD_URL) -o $(YQ) || { \
		echo "Failed to download yq from $(YQ_DOWNLOAD_URL). Please check the URL and your network connection."; \
		exit 1; \
	}
	@chmod +x $(YQ)
	@echo "yq downloaded successfully to $(YQ)."

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

default: build test

install:
	"$(GOCMD)" mod download

run:
	go run ./main.go

build:
	"$(GOCMD)" build ${GOFLAGS} ${LDFLAGS} -o "${BINARY}"

lint: golangci-lint ## Run golangci-lint on the codebase
	$(GOLANGCI_LINT) run ./...

build-image:
	docker buildx build \
		--platform ${OS}/${ARCH} \
		--build-arg GOARCH=$(ARCH) \
		--build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
		--build-arg BASE_IMAGE=${BASE_IMAGE} \
		--build-arg GOPROXY=${GOPROXY} \
		--build-arg GOPRIVATE=${GOPRIVATE} \
		-t "${REPOSITORY_ARCH}" \
		--load \
		-f Dockerfile \
		.

push:
	docker push ${REPOSITORY_ARCH}

release: build-image push manifest

release-all:
	-rm -rf ~/.docker/manifests/*
	# Make arch-specific release
	@for arch in $(ALL_ARCH) ; do \
		echo Make release: $$arch ; \
		make release ARCH=$$arch ; \
	done

	set -e
	docker manifest push --purge $(REPOSITORY_GENERIC)

manifest:
	set -e
	docker manifest create -a $(REPOSITORY_GENERIC) $(REPOSITORY_ARCH)
	docker manifest annotate --arch $(ARCH) $(REPOSITORY_GENERIC)  $(REPOSITORY_ARCH)

test:
	"$(GOCMD)" test -timeout 1800s -v -short -count=1 ./internal/... ./test/e2e/utils/...

##@ E2E Tests

E2E_IMG ?= ghcr.io/stakater/reloader:test
E2E_TIMEOUT ?= 45m
KIND_CLUSTER ?= reloader-e2e
CONTAINER_RUNTIME ?= $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)

.PHONY: e2e-setup
e2e-setup: ## One-time setup: create Kind cluster and install dependencies (Argo, CSI, Vault)
	@if kind get clusters 2>/dev/null | grep -q "^$(KIND_CLUSTER)$$"; then \
		echo "Kind cluster $(KIND_CLUSTER) already exists"; \
	else \
		echo "Creating Kind cluster $(KIND_CLUSTER)..."; \
		kind create cluster --name $(KIND_CLUSTER); \
	fi
	./scripts/e2e-cluster-setup.sh

.PHONY: e2e
e2e: ## Run e2e tests (builds image, loads to Kind, runs tests in parallel)
	$(CONTAINER_RUNTIME) build -t $(E2E_IMG) -f Dockerfile .
ifeq ($(notdir $(CONTAINER_RUNTIME)),podman)
	$(CONTAINER_RUNTIME) save $(E2E_IMG) -o /tmp/reloader-e2e.tar
	kind load image-archive /tmp/reloader-e2e.tar --name $(KIND_CLUSTER)
	rm -f /tmp/reloader-e2e.tar
else
	kind load docker-image $(E2E_IMG) --name $(KIND_CLUSTER)
endif
	SKIP_BUILD=true RELOADER_IMAGE=$(E2E_IMG) "$(GOCMD)" tool ginkgo --keep-going -v --timeout=$(E2E_TIMEOUT) ./test/e2e/...

.PHONY: e2e-cleanup
e2e-cleanup: ## Cleanup: remove test resources and delete Kind cluster
	./scripts/e2e-cluster-cleanup.sh
	kind delete cluster --name $(KIND_CLUSTER)

.PHONY: e2e-ci
e2e-ci: e2e-setup e2e e2e-cleanup ## CI pipeline: setup, run tests, cleanup

.PHONY: docker-build
docker-build: ## Build Docker image
	$(CONTAINER_RUNTIME) build -t $(IMG) -f Dockerfile .

stop:
	@docker stop "${BINARY}"

apply:
	kubectl apply -f deployments/manifests/ -n temp-reloader

deploy: binary-image push apply

.PHONY: k8s-manifests
k8s-manifests: $(KUSTOMIZE) ## Generate k8s manifests using Kustomize from 'manifests' folder
	$(KUSTOMIZE) build ./deployments/kubernetes/ -o ./deployments/kubernetes/reloader.yaml

.PHONY: update-manifests-version
update-manifests-version: ## Generate k8s manifests using Kustomize from 'manifests' folder
	sed -i 's/image:.*/image: \"ghcr.io\/stakater\/reloader:v$(VERSION)"/g' deployments/kubernetes/manifests/deployment.yaml

YQ_VERSION = v4.42.1
YQ_BIN = $(shell pwd)/yq
CURRENT_ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

YQ_DOWNLOAD_URL = "https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_linux_$(CURRENT_ARCH)"

yq-install:
	@echo "Downloading yq $(YQ_VERSION) for linux/$(CURRENT_ARCH)"
	@curl -sL $(YQ_DOWNLOAD_URL) -o $(YQ_BIN)
	@chmod +x $(YQ_BIN)
	@echo "yq $(YQ_VERSION) installed at $(YQ_BIN)"

# =============================================================================
# Load Testing
# =============================================================================

LOADTEST_BIN = test/loadtest/loadtest
LOADTEST_OLD_IMAGE ?= localhost/reloader:old
LOADTEST_NEW_IMAGE ?= localhost/reloader:new
LOADTEST_DURATION ?= 60
LOADTEST_SCENARIOS ?= all

.PHONY: loadtest-build loadtest-quick loadtest-full loadtest loadtest-clean

loadtest-build: ## Build loadtest binary
	cd test/loadtest && $(GOCMD) build -o loadtest ./cmd/loadtest

loadtest-quick: loadtest-build ## Run quick load tests (S1, S4, S6)
	cd test/loadtest && ./loadtest run \
		--old-image=$(LOADTEST_OLD_IMAGE) \
		--new-image=$(LOADTEST_NEW_IMAGE) \
		--scenario=S1,S4,S6 \
		--duration=$(LOADTEST_DURATION)

loadtest-full: loadtest-build ## Run full load test suite
	cd test/loadtest && ./loadtest run \
		--old-image=$(LOADTEST_OLD_IMAGE) \
		--new-image=$(LOADTEST_NEW_IMAGE) \
		--scenario=all \
		--duration=$(LOADTEST_DURATION)

loadtest: loadtest-build ## Run load tests with configurable scenarios (default: all)
	cd test/loadtest && ./loadtest run \
		--old-image=$(LOADTEST_OLD_IMAGE) \
		--new-image=$(LOADTEST_NEW_IMAGE) \
		--scenario=$(LOADTEST_SCENARIOS) \
		--duration=$(LOADTEST_DURATION)

loadtest-clean: ## Clean loadtest binary and results
	rm -f $(LOADTEST_BIN)
	rm -rf test/loadtest/results
