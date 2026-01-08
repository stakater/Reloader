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
GOPROXY   ?=
GOPRIVATE ?=

# Version information for ldflags
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -s -w \
	-X github.com/stakater/Reloader/internal/pkg/metadata.Version=$(VERSION) \
	-X github.com/stakater/Reloader/internal/pkg/metadata.Commit=$(GIT_COMMIT) \
	-X github.com/stakater/Reloader/internal/pkg/metadata.BuildDate=$(BUILD_DATE)

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
YQ ?= $(LOCALBIN)/yq

## Tool Versions
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

default: build test

install:
	"$(GOCMD)" mod download

run:
	go run ./cmd/reloader

build:
	"$(GOCMD)" build ${GOFLAGS} -ldflags '${LDFLAGS}' -o "${BINARY}" ./cmd/reloader

lint: ## Run golangci-lint on the codebase
	go tool golangci-lint run ./...

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
	"$(GOCMD)" test -timeout 1800s -v -short ./cmd/... ./internal/...

.PHONY: docker-build
docker-build: ## Build Docker image
	$(CONTAINER_RUNTIME) build -t $(IMG) -f Dockerfile .

stop:
	@docker stop "${BINARY}"

apply:
	kubectl apply -f deployments/manifests/ -n temp-reloader

deploy: binary-image push apply

.PHONY: k8s-manifests
k8s-manifests: ## Generate k8s manifests using Kustomize from 'manifests' folder
	go tool kustomize build ./deployments/kubernetes/ -o ./deployments/kubernetes/reloader.yaml

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
