GO=$(shell which go)
GIT=$(shell which git)
kubernetesVer=$(shell $(GO) list -m k8s.io/api | cut -d' ' -f 2)

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

.PHONY: check-k8s-version
check-k8s-version:
	@echo "Checking kubectl version in Dockerfile against k8s.io/api version: $(kubernetesVer)"; \
	expected_kubectl_version=$(kubernetesVer); \
	if grep -q "$$expected_kubectl_version" Dockerfile; then \
		echo "kubectl version in Dockerfile matches k8s.io/api version: $$expected_kubectl_version"; \
	else \
		echo "ERROR: kubectl version in Dockerfile does not match k8s.io/api version. Expected: $$expected_kubectl_version"; \
		exit 1; \
	fi

define error-if-empty
@if [[ -z $(1) ]]; then echo "$(2) not installed"; false; fi
endef