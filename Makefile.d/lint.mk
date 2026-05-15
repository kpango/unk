
.PHONY: lint
lint: vet go/lint ## run all linters (go vet + golangci-lint)

.PHONY: go/lint
go/lint: golangci-lint/install ## run golangci-lint with auto-fix
	@$(call green, "Running golangci-lint...")
	$(call go-lint)
	@$(call green, "golangci-lint complete.")

.PHONY: go/lint/nofix
go/lint/nofix: golangci-lint/install ## run golangci-lint without auto-fix (CI mode)
	@$(call green, "Running golangci-lint (no fix)...")
	$(call go-lint-nofix)
	@$(call green, "golangci-lint complete.")

.PHONY: vet
vet: ## run go vet
	@$(call green, "Running go vet...")
	$(call go-vet)
	@$(call green, "go vet complete.")

.PHONY: staticcheck
staticcheck: staticcheck/install ## run staticcheck
	@$(call green, "Running staticcheck...")
	$(GOENV) $(GOBIN)/staticcheck $(ROOTDIR)/...
	@$(call green, "staticcheck complete.")
