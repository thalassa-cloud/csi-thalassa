BINARY = csi-thalassa-plugin
GOARCH = amd64

IMAGE 		?=ghcr.io/thalassa-cloud/csi-thalassa
VERSION		?=local
COMMIT		?=$(shell git rev-parse HEAD)
BUILD_DATE	?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_BY 	?=make

E2E_REGISTRY           ?= registry.thalassacloud.nl
E2E_IMAGE_REPOSITORY   ?= csi-thalassa-dev/csi-thalassa
E2E_IMAGE_TAG          ?= e2e-$(COMMIT)
E2E_IMAGE              ?= $(E2E_REGISTRY)/$(E2E_IMAGE_REPOSITORY):$(E2E_IMAGE_TAG)
E2E_RUN_ID             ?= local-$(shell date +%s)
E2E_REGION             ?= nl-01
E2E_AVAILABILITY_ZONE  ?= nl-01a
E2E_MACHINE_TYPE       ?= pgp-medium
E2E_NODE_COUNT         ?= 2
E2E_POD_SECURITY_STANDARDS ?= privileged
E2E_KUBECONFIG         := $(CURDIR)/test/e2e/kubeconfig

# Optional local credentials for e2e targets (gitignored .env at repo root).
ifneq (,$(wildcard .env))
include .env
export
endif

export E2E_REGISTRY E2E_IMAGE_REPOSITORY E2E_IMAGE_TAG E2E_IMAGE
export E2E_RUN_ID E2E_REGION E2E_AVAILABILITY_ZONE E2E_MACHINE_TYPE E2E_NODE_COUNT E2E_POD_SECURITY_STANDARDS

PKG_LIST := $(shell go list ./... | grep -v /vendor/)

LDFLAGS = -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE} -X main.builtBy="${BUILD_BY}"

compile: build

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH} go build  -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_linux_${GOARCH}/${BINARY} ./cmd/thalassa-csi-plugin/ ;

darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=${GOARCH} go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY}_darwin_${GOARCH}/${BINARY} ./cmd/thalassa-csi-plugin/ ;

build:
	CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ./bin/${BINARY} ./cmd/thalassa-csi-plugin/ ;

snapshot:
	goreleaser release --clean --snapshot --skip=validate

lint: ## Lint the files
	@golint -set_exit_status ${PKG_LIST}

test: ## Run unittests
	@go test -short ${PKG_LIST}

race: ## Run data race detector
	@go test -race -short ${PKG_LIST}

fmt:
	@go fmt ${PKG_LIST};

docker: linux
	docker build -t ${IMAGE}:${VERSION}${BRANCH} .

e2e-image-build: ## Build the container image used by e2e tests
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o csi-thalassa ./cmd/thalassa-csi-plugin/
	docker build --platform linux/amd64 -t $(E2E_IMAGE) .
	rm -f csi-thalassa

e2e-image-push: ## Build and push the container image used by e2e tests
	E2E_REGISTRY='$(E2E_REGISTRY)' \
	E2E_IMAGE_REPOSITORY='$(E2E_IMAGE_REPOSITORY)' \
	E2E_IMAGE_TAG='$(E2E_IMAGE_TAG)' \
	E2E_IMAGE='$(E2E_IMAGE)' \
	bash test/e2e/scripts/build-push-image.sh

e2e-prepare: ## Install tcloud and provision a Thalassa Kubernetes cluster for e2e
	bash test/e2e/scripts/install-tcloud.sh
	bash test/e2e/scripts/bootstrap-cluster.sh
	@echo "E2e cluster ready. Run: export KUBECONFIG=$(E2E_KUBECONFIG)"

e2e-deploy: ## Deploy the CSI driver to the e2e cluster
	KUBECONFIG='$(E2E_KUBECONFIG)' \
	E2E_REGISTRY='$(E2E_REGISTRY)' \
	E2E_IMAGE_REPOSITORY='$(E2E_IMAGE_REPOSITORY)' \
	E2E_IMAGE_TAG='$(E2E_IMAGE_TAG)' \
	E2E_IMAGE='$(E2E_IMAGE)' \
	bash test/e2e/scripts/deploy-driver.sh

e2e-test: ## Run Kubernetes external storage e2e tests against the e2e cluster
	KUBECONFIG='$(E2E_KUBECONFIG)' \
	bash test/e2e/e2e.sh

e2e-test-smoke: ## Run a shorter e2e subset (ext4 dynamic PV provisioning)
	KUBECONFIG='$(E2E_KUBECONFIG)' \
	E2E_TEST_PROFILE='smoke' \
	bash test/e2e/e2e.sh

e2e-teardown: ## Tear down Thalassa Cloud resources created for e2e
	bash test/e2e/scripts/teardown-cluster.sh

clean:
	-rm -f bin/${BINARY}-* bin/${BINARY}

review:
	reviewdog -diff="git diff FETCH_HEAD" -tee

.PHONY: linux darwin build lint test fmt clean review e2e-image-build e2e-image-push e2e-prepare e2e-deploy e2e-test e2e-test-smoke e2e-teardown
