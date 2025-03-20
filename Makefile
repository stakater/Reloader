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
GOLANGCI_LINT_VERSION ?= v1.57.2

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
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

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
	"$(GOCMD)" test -timeout 1800s -v ./...

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

# Bump Chart
bump-chart:
	sed -i "s/^appVersion:.*/appVersion: v$(VERSION)/" deployments/kubernetes/chart/reloader/Chart.yaml
	sed -i "s/tag:.*/tag: v$(VERSION)/" deployments/kubernetes/chart/reloader/values.yaml
	sed -i "s/version:.*/version: v$(VERSION)/" deployments/kubernetes/chart/reloader/values.yaml

YQ_VERSION = v4.42.1
YQ_BIN = $(shell pwd)/yq
CURRENT_ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

YQ_DOWNLOAD_URL = "https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_linux_$(CURRENT_ARCH)"

yq-install:
	@echo "Downloading yq $(YQ_VERSION) for linux/$(CURRENT_ARCH)"
	@curl -sL $(YQ_DOWNLOAD_URL) -o $(YQ_BIN)
	@chmod +x $(YQ_BIN)
	@echo "yq $(YQ_VERSION) installed at $(YQ_BIN)"
