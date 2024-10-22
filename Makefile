export APP_NAME=intel-kubernetes-power-manager

CONTROLLER_IMG ?= intel/power-operator
AGENT_IMG ?= intel/power-node-agent

# Prepend registry address to image tags for controller and agent.
# The address is sourced from IMAGE_REGISTRY environment variable.
ifneq (, $(IMAGE_REGISTRY))
CONTROLLER_IMG_BASE = $(IMAGE_REGISTRY)/$(CONTROLLER_IMG)
AGENT_IMG_BASE = $(IMAGE_REGISTRY)/$(AGENT_IMG)
else
CONTROLLER_IMG_BASE = $(CONTROLLER_IMG)
AGENT_IMG_BASE = $(AGENT_IMG)
endif

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 2.5.0

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# my.domain/blank-bundle:$VERSION and my.domain/blank-catalog:$VERSION.
ifneq (, $(IMAGE_REGISTRY))
IMAGE_TAG_BASE = $(IMAGE_REGISTRY)/$(APP_NAME)
else
IMAGE_TAG_BASE = $(APP_NAME)
endif

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.37.0
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.29.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# parameters used for helm chart images
HELM ?= helm
HELM_CHART ?= v2.5.0
HELM_VERSION := $(shell echo $(HELM_CHART) | cut -d "v" -f2)

# used to detemine if certain targets should build for openshift
OCP ?= false
# version of ocp being supported
OCP_VERSION=4.13
# image used for building the dockerfile for ocp
OCP_IMAGE=redhat/ubi9-minimal@sha256:06d06f15f7b641a78f2512c8817cbecaa1bf549488e273f5ac27ff1654ed33f0

TLS_VERIFY ?= false

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

BUNDLE_IMGS ?= $(BUNDLE_IMG)

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
ifeq (false, $(OCP))
	sed -i 's/- .*\/rbac\.yaml/- \.\/rbac.yaml/' config/rbac/kustomization.yaml
	sed -i 's/- .*\/role\.yaml/- \.\/role.yaml/' config/rbac/kustomization.yaml
else
	sed -i 's/- .*\/rbac\.yaml/- \.\/ocp\/rbac.yaml/' config/rbac/kustomization.yaml
	sed -i 's/- .*\/role\.yaml/- \.\/ocp\/role.yaml/' config/rbac/kustomization.yaml
endif
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet -composites=false ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

.PHONY: tls
tls: ## Test the generation of TLS certificates
	./build/gen_test_certs.sh

.PHONY: coverage
coverage: ## Run coverage tests.
	go test -v -coverprofile=coverage.out ./controllers/ ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Average code coverage: $$(go tool cover -func coverage.out | awk 'END {print $$3}' | sed 's/\..*//')%" 
	@if [ $$(go tool cover -func coverage.out | awk 'END {print $$3}' | sed 's/\..*//') -lt 85 ]; then \
                echo "Total unit test coverage below 85%"; false; \
        fi

.PHONY: tidy
tidy:	## Run go mod tidy
	go mod tidy

.PHONY: verify-test
verify-test: tidy ## Run Go tests without cache
	CGO_ENABLED=1 go test -count=1 -v ./...

.PHONY: race
race: tidy ## Run Go race condition tests without cache
	CGO_ENABLED=1 go test -count=1 -race -v ./...

.PHONY: gofmt
gofmt: # Run gofmt
	gofmt -w .

.PHONY: update
update: ## Update project version
	sed -i 's|image: .*|image: $(CONTROLLER_IMG_BASE):v$(VERSION)|' config/manager/manager.yaml
	sed -i 's|image: .*|image: $(CONTROLLER_IMG_BASE)_ocp-$(OCP_VERSION):v$(VERSION)|' config/manager/ocp/manager.yaml
	sed -i 's|image: .*|image: $(AGENT_IMG_BASE):v$(VERSION)|' build/manifests/power-node-agent-ds.yaml
	sed -i 's|image: .*|image: $(AGENT_IMG_BASE)_ocp-$(OCP_VERSION):v$(VERSION)|' build/manifests/ocp/power-node-agent-ds.yaml

.PHONY: clean
clean: # Clean Go cache and build artifacts
	go clean --cache
	rm -r build/bin/manager
	rm -r build/bin/nodeagent

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/manager build/manager/main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/nodeagent build/nodeagent/main.go

.PHONY: verify-build  ## Build manager binary and run tests.
verify-build: gofmt test race coverage tidy clean verify-test
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/manager build/manager/main.go
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/nodeagent build/nodeagent/main.go	

.PHONY: run
run: generate fmt vet manifests ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: build-controller
build-controller: ## Build the Manager's image.
	$(CONTAINER_TOOL) build -f build/Dockerfile -t $(CONTROLLER_IMG_BASE):v$(VERSION) .

.PHONY: build-agent
build-agent: ## Build the Node Agent's image.
	$(CONTAINER_TOOL) build -f build/Dockerfile.nodeagent -t $(AGENT_IMG_BASE):v$(VERSION) .

.PHONY: build-controller-ocp
build-controller-ocp: ## Build the Manager's image for OCP.
	$(CONTAINER_TOOL) build --build-arg="BASE_IMAGE=$(OCP_IMAGE)" --build-arg="MANIFEST=build/manifests/ocp/power-node-agent-ds.yaml" -f build/Dockerfile -t $(CONTROLLER_IMG_BASE)_ocp-$(OCP_VERSION):v$(VERSION) .

.PHONY: build-agent-ocp
build-agent-ocp: ## Build the Node Agent's image for OCP.
	$(CONTAINER_TOOL) build --build-arg="BASE_IMAGE=$(OCP_IMAGE)" -f build/Dockerfile.nodeagent -t $(AGENT_IMG_BASE)_ocp-$(OCP_VERSION):v$(VERSION) .

.PHONY: images
images: generate manifests build-controller build-agent ## Build the Manager and Node Agent images.

.PHONY: docker-build
docker-build: images ## Alias for images target.

.PHONY: images-ocp
images-ocp: generate manifests build-controller-ocp build-agent-ocp ## Build the Manager and Node Agent images for OCP.

.PHONY: push
push: ## Push docker images with the controller and agent.
	$(MAKE) docker-push IMG=$(CONTROLLER_IMG_BASE):v$(VERSION)
	$(MAKE) docker-push IMG=$(AGENT_IMG_BASE):v$(VERSION)

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le

.PHONY: buildx-controller
buildx-controller:  ## Build and push docker image for the Manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' build/Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: buildx-agent
buildx-agent:  ## Build and push docker image for the Agent for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' build/Dockerfile.nodeagent > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: docker-buildx
docker-buildx: buildx-agent buildx-agent ## Build and push docker image for the Manager and Agent for cross-platform support

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: helm-install
helm-install: ## Install project helm charts.
ifeq (true, $(OCP))
	$(eval HELM_FLAG:=--set ocp=true)
	$(eval OCP_SUFFIX:=_ocp-$(OCP_VERSION))
endif
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/kubernetes-power-manager/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/kubernetes-power-manager/Chart.yaml
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/crds/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/crds/Chart.yaml 
	$(HELM) install kubernetes-power-manager-crds ./helm/crds
	$(HELM) dependency update ./helm/kubernetes-power-manager
	$(HELM) install kubernetes-power-manager-$(HELM_CHART) ./helm/kubernetes-power-manager --set operator.container.image=$(CONTROLLER_IMG_BASE)$(OCP_SUFFIX):$(HELM_CHART) $(HELM_FLAG)

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall project helm charts.
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/kubernetes-power-manager/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/kubernetes-power-manager/Chart.yaml 
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/crds/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/crds/Chart.yaml 
	$(HELM) uninstall kubernetes-power-manager-$(HELM_CHART)
	$(HELM) uninstall kubernetes-power-manager-crds

##@ Dependencies

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

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.14.0
ENVTEST_VERSION ?= release-0.17
GOLANGCI_LINT_VERSION ?= v1.57.2

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

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
ifeq (false, $(OCP))
	sed -i 's/\.\.\/manager.*$$/\.\.\/manager/' config/default/kustomization.yaml
else
	sed -i 's/\.\.\/manager.*$$/\.\.\/manager\/ocp/' config/default/kustomization.yaml
	sed -i 's/- .*rbac\.yaml/- \.\/ocp\/rbac.yaml/' config/rbac/kustomization.yaml
	sed -i 's/- .*role\.yaml/- \.\/ocp\/role.yaml/' config/rbac/kustomization.yaml
endif
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	$(CONTAINER_TOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = $(LOCALBIN)/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool $(CONTAINER_TOOL) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(if ifeq $(TLS_VERIFY) false, --skip-tls) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)
