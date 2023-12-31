# Copyright (c) 2023 Red Hat, Inc.

###############################################
###### MultiCluster Resiliency ACM Addon ######
###############################################
default: help

#####################################
###### Image related variables ######
#####################################
IMAGE_BUILDER ?= podman##@ Set a custom image builder if 'podman' is not available
IMAGE_REGISTRY ?= quay.io##@ Set the image registry for build and config, defaults to 'quay.io'
IMAGE_NAMESPACE ?= ecosystem-appeng##@ Set the image namespace for build and config, defaults to 'ecosystem-appeng'
IMAGE_NAME ?= multicluster-resiliency-addon##@ Set the image name for build and config, defaults to 'multicluster-resiliency-addon'
IMAGE_TAG ?= $(strip $(shell cat VERSION))##@ Set the image tag for build and config, defaults to content of the VERSION file

##########################################################
###### Create working directories (note .gitignore) ######
##########################################################
LOCALBIN = $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

LOCALBUILD = $(shell pwd)/build
$(LOCALBUILD):
	mkdir -p $(LOCALBUILD)

#####################################
###### Tool binaries variables ######
#####################################
BIN_CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen##@ Set custom 'controller-gen', if not supplied will install latest in ./bin
BIN_KUSTOMIZE ?= $(LOCALBIN)/kustomize##@ Set custom 'kustomize', if not supplied will install latest in ./bin
BIN_GREMLINS ?= $(LOCALBIN)/gremlins##@ Set custom 'gremlins', if not supplied will install latest in ./bin
BIN_GO_TEST_COVERAGE ?= $(LOCALBIN)/go-test-coverage##@ Set custom 'go-test-coverage', if not supplied will install latest in ./bin
BIN_GOLINTCI ?= $(LOCALBIN)/golangci-lint##@ Set custom 'golangci-lint', if not supplied will install latest in ./bin
BIN_ACTIONLINT ?= $(LOCALBIN)/actionlint##@ Set custom 'actionlint', if not supplied will install latest in ./bin
BIN_AWK ?= awk##@ Set a custom 'awk' binary path if not in PATH
BIN_OC ?= oc##@ Set a custom 'oc' binary path if not in PATH

###############################
###### Various variables ######
###############################
SPOKE_NAME ?= vp-pool-m2rdd##@ Set the name of the Spoke to install the addon manager in, defaults to 'cluster1'
COVERAGE_THRESHOLD ?= 60##@ Set the unit test code coverage threshold, defaults to '60'

#########################
###### Build times ######
#########################
BUILD_DATE = $(strip $(shell date +%FT%T))
BUILD_TIMESTAMP = $(strip $(shell date -d "$(BUILD_DATE)" +%s))

#########################
###### Build flags ######
#########################
COMMIT_HASH = $(strip $(shell git rev-parse --short HEAD))
LDFLAGS=-ldflags="\
-X 'github.com/rhecosystemappeng/multicluster-resiliency-addon/pkg/version.tag=${IMAGE_TAG}' \
-X 'github.com/rhecosystemappeng/multicluster-resiliency-addon/pkg/version.commit=${COMMIT_HASH}' \
-X 'github.com/rhecosystemappeng/multicluster-resiliency-addon/pkg/version.date=${BUILD_DATE}' \
"

#########################
###### Image names ######
#########################
FULL_IMAGE_NAME = $(strip $(IMAGE_REGISTRY)/$(IMAGE_NAMESPACE)/$(IMAGE_NAME):$(IMAGE_TAG))
FULL_IMAGE_NAME_UNIQUE = $(FULL_IMAGE_NAME)_$(COMMIT_HASH)_$(BUILD_TIMESTAMP)

####################################
###### Build and push project ######
####################################
.PHONY: build
build: $(LOCALBUILD) ## Build the project as a binary in ./build
	go build $(LDFLAGS) -o $(LOCALBUILD)/mcra ./main.go

.PHONY: build/image
build/image: ## Build the image, customized with IMAGE_REGISTRY, IMAGE_NAMESPACE, IMAGE_NAME, and IMAGE_TAG
	$(IMAGE_BUILDER) build --ignorefile ./.gitignore --tag $(FULL_IMAGE_NAME) -f ./Containerfile

build/image/push: build/image ## Build and push the image, customized with IMAGE_REGISTRY, IMAGE_NAMESPACE, IMAGE_NAME, and IMAGE_TAG
	$(IMAGE_BUILDER) tag $(FULL_IMAGE_NAME) $(FULL_IMAGE_NAME_UNIQUE)
	$(IMAGE_BUILDER) push $(FULL_IMAGE_NAME_UNIQUE)
	$(IMAGE_BUILDER) push $(FULL_IMAGE_NAME)

###########################################
###### Code and Manifests generation ######
###########################################
generate/all: generate/manifests generate/code ## Generate both the code and the manifests

.PHONY: generate/manifests
generate/manifests: $(BIN_CONTROLLER_GEN) ## Generate the manifest files
	$(BIN_CONTROLLER_GEN) rbac:roleName=role paths="./pkg/controllers/reconcilers/..."
	$(BIN_CONTROLLER_GEN) crd paths="./api/..."
	$(BIN_CONTROLLER_GEN) webhook paths="./pkg/controllers/webhooks/..."

.PHONY: generate/code
generate/code: $(BIN_CONTROLLER_GEN) ## Generate API boiler-plate code
	rm -rf ./api/**/zz_generated.deepcopy.go
	$(BIN_CONTROLLER_GEN) object:headerFile="hack/header.txt" paths="./api/..."

########################################
###### Deploy and Apply resources ######
########################################
addon/deploy: $(BIN_KUSTOMIZE) verify/tools/oc ## Deploy the addon manager on the Hub cluster
	cp config/addon/kustomization.yaml config/addon/kustomization.yaml.tmp
	cd config/addon && $(BIN_KUSTOMIZE) edit set image manager-image=$(FULL_IMAGE_NAME)
	$(BIN_KUSTOMIZE) build config/default | $(BIN_OC) apply -f -
	mv config/addon/kustomization.yaml.tmp config/addon/kustomization.yaml

addon/undeploy: $(BIN_KUSTOMIZE) verify/tools/oc ## Remove the addon manager on the Hub cluster
	cp config/addon/kustomization.yaml config/addon/kustomization.yaml.tmp
	cd config/addon && $(BIN_KUSTOMIZE) edit set image manager-image=$(FULL_IMAGE_NAME)
	$(BIN_KUSTOMIZE) build config/default | $(BIN_OC) delete --ignore-not-found -f -
	mv config/addon/kustomization.yaml.tmp config/addon/kustomization.yaml

addon/install: $(BIN_KUSTOMIZE) verify/tools/oc ## Install the addon agent for a spoke named in SPOKE_NAME on the Hub cluster
	cp config/samples/kustomization.yaml config/samples/kustomization.yaml.tmp
	cd config/samples && $(BIN_KUSTOMIZE) edit set namespace $(SPOKE_NAME)
	$(BIN_KUSTOMIZE) build config/samples | $(BIN_OC) apply -f -
	mv config/samples/kustomization.yaml.tmp config/samples/kustomization.yaml

addon/uninstall: $(BIN_KUSTOMIZE) verify/tools/oc ## Remove the addon agent for a spoke named in SPOKE_NAME on the Hub cluster
	cp config/samples/kustomization.yaml config/samples/kustomization.yaml.tmp
	cd config/samples && $(BIN_KUSTOMIZE) edit set namespace $(SPOKE_NAME)
	$(BIN_KUSTOMIZE) build config/samples | $(BIN_OC) delete --ignore-not-found -f -
	mv config/samples/kustomization.yaml.tmp config/samples/kustomization.yaml

###########################
###### Test codebase ######
###########################
.PHONY: test
test: ## Run all unit tests
	go test -v ./...

.PHONY: test/cov
test/cov: $(BIN_GO_TEST_COVERAGE) ## Run all unit tests and print coverage report, use the COVERAGE_THRESHOLD var for setting threshold
	go test -failfast -coverprofile=cov.out -v ./...
	go tool cover -func=cov.out
	go tool cover -html=cov.out -o cov.html
	$(BIN_GO_TEST_COVERAGE) -p cov.out -k 0 -t $(COVERAGE_THRESHOLD)

.PHONY: test/mut
test/mut: $(BIN_GREMLINS) ## Run mutation tests
	$(BIN_GREMLINS) unleash

###########################
###### Lint codebase ######
###########################
lint/all: lint/code lint/ci lint/containerfile ## Lint the entire project (code, ci, containerfile)

.PHONY: lint/code
lint/code: $(BIN_GOLINTCI) ## Lint the code
	go fmt ./...
	$(BIN_GOLINTCI) run

.PHONY: lint/ci
lint/ci: $(BIN_ACTIONLINT) ## Lint the ci
	$(BIN_ACTIONLINT) --verbose

.PHONY: lint/containerfile
lint/containerfile: ## Lint the Containerfile (using Hadolint image, do not use inside a container)
	$(IMAGE_BUILDER) run --rm -i docker.io/hadolint/hadolint:latest < ./Containerfile

################################
###### Display build help ######
################################
help: verify/tools/awk ## Show this help message
	@$(BIN_AWK) 'BEGIN {\
			FS = ".*##@";\
			print "\033[1;31mMulticluster Resiliency Addon\033[0m";\
			print "\033[1;32mUsage\033[0m";\
			printf "\t\033[1;37mmake <target> |";\
			printf "\tmake <target> [Variables Set] |";\
            printf "\tmake [Variables Set] <target> |";\
            print "\t[Variables Set] make <target>\033[0m";\
			print "\033[1;32mAvailable Variables\033[0m" }\
		/^(\s|[a-zA-Z_0-9-]|\/)+ \?=.*?##@/ {\
			split($$0,t,"?=");\
			printf "\t\033[1;36m%-35s \033[0;37m%s\033[0m\n",t[1], $$2 | "sort" }'\
		$(MAKEFILE_LIST)
	@$(BIN_AWK) 'BEGIN {\
			FS = ":.*##";\
			SORTED = "sort";\
            print "\033[1;32mAvailable Targets\033[0m"}\
		/^(\s|[a-zA-Z_0-9-]|\/)+:.*?##/ {\
			if($$0 ~ /addon/)\
				printf "\t\033[1;36m%-35s \033[0;33m%s\033[0m\n", $$1, $$2 | SORTED;\
			else\
				printf "\t\033[1;36m%-35s \033[0;37m%s\033[0m\n", $$1, $$2 | SORTED; }\
		END { \
			close(SORTED);\
			print "\033[1;32mFurther Information\033[0m";\
			print "\t\033[0;37m* Source code: \033[38;5;26mhttps://github.com/RHEcosystemAppEng/multicluster-resiliency-addon\33[0m"}'\
		$(MAKEFILE_LIST)

####################################
###### Install required tools ######
####################################
$(BIN_KUSTOMIZE):
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v5@latest

$(BIN_CONTROLLER_GEN):
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

$(BIN_GREMLINS):
	GOBIN=$(LOCALBIN) go install github.com/go-gremlins/gremlins/cmd/gremlins@latest

$(BIN_GO_TEST_COVERAGE):
	GOBIN=$(LOCALBIN) go install github.com/vladopajic/go-test-coverage/v2@latest

$(BIN_GOLINTCI):
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

$(BIN_ACTIONLINT): # recommendation: manually install shellcheck and verify it's on your PATH, it will be picked up by actionlint
	GOBIN=$(LOCALBIN) go install github.com/rhysd/actionlint/cmd/actionlint@latest

######################################
###### Verify tools availablity ######
######################################

# member 1 is the missing tool name, member 2 is the name of the variable used for customizing the tool path
TOOL_MISSING_ERR_MSG = Please install '$(1)' or specify a custom path using the '$(2)' variable

.PHONY: verify/tools/awk
verify/tools/awk:
ifeq (,$(shell which $(BIN_AWK) 2> /dev/null ))
	$(error $(call TOOL_MISSING_ERR_MSG,awk,BIN_AWK))
endif

.PHONY: verify/tools/oc
verify/tools/oc:
ifeq (,$(shell which $(BIN_OC) 2> /dev/null ))
	$(error $(call TOOL_MISSING_ERR_MSG,oc,BIN_OC))
endif
