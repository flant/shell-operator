GO=$(shell which go)
GIT=$(shell which git)

.PHONY: go-check
go-check:
	$(call error-if-empty,$(GO),go)

.PHONY: git-check
git-check:
	$(call error-if-empty,$(GIT),git)

.PHONY: go-module-version
go-module-version: go-check git-check
	@echo "go get $(shell $(GO) list ./cmd/shell-operator)@$(shell $(GIT) rev-parse HEAD)"

.PHONY: test
test: go-check
	@$(GO) test --race --cover ./...

## Run all generate-* jobs in bulk.
.PHONY: generate
generate: update-k8s-version update-workflows-go-version update-workflows-golangci-lint-version


##@ Dependencies

WHOAMI ?= $(shell whoami)

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
YQ = $(LOCALBIN)/yq

## TODO: remap in yaml file (version.yaml or smthng)
## Tool Versions
# GO_BUILDER_VERSION must be without 'v' prefix
GO_BUILDER_VERSION = 1.25.5
GOLANGCI_LINT_VERSION = v2.7.2
YQ_VERSION ?= v4.47.2

.PHONY: update-k8s-version
update-k8s-version: go-check
	@kubernetesVer=$(shell $(GO) list -m k8s.io/api | cut -d' ' -f 2); \
	echo "Updating kubectl version in Dockerfile to match k8s.io/api version: $$kubernetesVer"; \
	sed -i "s/ARG kubectlVersion=.*/ARG kubectlVersion=$$kubernetesVer/" Dockerfile; \
	echo "kubectl version in Dockerfile updated to: $$kubernetesVer"
	
.PHONY: update-workflows-go-version
update-workflows-go-version: yq
	for file in $$(find .github/workflows -name "*.yaml"); do \
		$(YQ) -i '(.jobs[]?.steps[]? | select(.uses | test("actions/setup-go")) | .with."go-version") = "$(GO_BUILDER_VERSION)"' $$file; \
	done
	echo "Updated go-version in workflow files to $(GO_BUILDER_VERSION)"

.PHONY: update-workflows-golangci-lint-version
update-workflows-golangci-lint-version: yq
	$(YQ) -i '(.jobs.run_linter.steps[] | select(.name == "Run golangci-lint") | .run) |= sub("v\\d+\\.\\d+\\.\\d+", "$(GOLANGCI_LINT_VERSION)")' .github/workflows/lint.yaml
	echo "Updated golangci-lint version in lint.yaml to $(GOLANGCI_LINT_VERSION)"

## Tool installations

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: yq
yq: $(YQ) ## Download yq locally if necessary.
$(YQ): $(LOCALBIN)
	$(call go-install-tool,$(YQ),github.com/mikefarah/yq/v4,$(YQ_VERSION))


# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) GOTOOLCHAIN=$(GO_TOOLCHAIN_AUTOINSTALL_VERSION) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef


define error-if-empty
@if [[ -z $(1) ]]; then echo "$(2) not installed"; false; fi
endef