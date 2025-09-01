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

define error-if-empty
@if [[ -z $(1) ]]; then echo "$(2) not installed"; false; fi
endef