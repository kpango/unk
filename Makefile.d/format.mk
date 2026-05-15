
.PHONY: format
format: format/go format/go/test ## format all Go source files

.PHONY: format/go
format/go: ## format non-test Go files (strictgoimports, crlfmt, golangci-lint fmt)
format/go: \
	strictgoimports/install \
	crlfmt/install \
	golangci-lint/install \
	files
	@$(call green, "Formatting Go files...")
	@if [ -f "$(ROOTDIR)/.gitfiles" ]; then \
		grep -e "\.go$$" "$(ROOTDIR)/.gitfiles" | grep -v "_test\.go$$" \
		| xargs $(XARGS_NO_RUN_IF_EMPTY) -I {} -P"$(CORES)" bash -c ' \
		echo "Formatting Go file {}" && \
		$(GOBIN)/strictgoimports -w {} && \
		$(GOBIN)/crlfmt -w -diff=false {} && \
		$(BINDIR)/golangci-lint fmt --config $(ROOTDIR)/.golangci.json {}'; \
	fi
	$(GOENV) go fix $(ROOTDIR)/...
	@$(call green, "Go formatting complete.")

.PHONY: format/go/test
format/go/test: ## format test Go files (strictgoimports, crlfmt, golangci-lint fmt)
format/go/test: \
	strictgoimports/install \
	crlfmt/install \
	golangci-lint/install \
	files
	@$(call green, "Formatting Go test files...")
	@if [ -f "$(ROOTDIR)/.gitfiles" ]; then \
		grep -e "_test\.go$$" "$(ROOTDIR)/.gitfiles" \
		| xargs $(XARGS_NO_RUN_IF_EMPTY) -I {} -P"$(CORES)" bash -c ' \
		echo "Formatting Go test file {}" && \
		$(GOBIN)/strictgoimports -w {} && \
		$(GOBIN)/crlfmt -w -diff=false {} && \
		$(BINDIR)/golangci-lint fmt --config $(ROOTDIR)/.golangci.json {}'; \
	fi
	@$(call green, "Go test file formatting complete.")

.PHONY: format/go/diff
format/go/diff: ## format only non-test Go files changed relative to HEAD
format/go/diff: \
	strictgoimports/install \
	crlfmt/install \
	golangci-lint/install
	@$(call green, "Formatting changed Go files...")
	@git diff --name-only --diff-filter=ACM HEAD | grep -e "\.go$$" | grep -v "_test\.go$$" \
		| xargs $(XARGS_NO_RUN_IF_EMPTY) -I {} -P"$(CORES)" bash -c ' \
		echo "Formatting Go file {}" && \
		$(GOBIN)/strictgoimports -w {} && \
		$(GOBIN)/crlfmt -w -diff=false {} && \
		$(BINDIR)/golangci-lint fmt --config $(ROOTDIR)/.golangci.json {}'
	@$(call green, "Changed Go file formatting complete.")

.PHONY: format/go/test/diff
format/go/test/diff: ## format only test Go files changed relative to HEAD
format/go/test/diff: \
	strictgoimports/install \
	crlfmt/install \
	golangci-lint/install
	@$(call green, "Formatting changed Go test files...")
	@git diff --name-only --diff-filter=ACM HEAD | grep -e "_test\.go$$" \
		| xargs $(XARGS_NO_RUN_IF_EMPTY) -I {} -P"$(CORES)" bash -c ' \
		echo "Formatting Go test file {}" && \
		$(GOBIN)/strictgoimports -w {} && \
		$(GOBIN)/crlfmt -w -diff=false {} && \
		$(BINDIR)/golangci-lint fmt --config $(ROOTDIR)/.golangci.json {}'
	@$(call green, "Changed Go test file formatting complete.")

.PHONY: format/check
format/check: golangci-lint/install ## check formatting without modifying files (CI mode)
	@$(call green, "Checking Go formatting...")
	$(BINDIR)/golangci-lint fmt --config $(ROOTDIR)/.golangci.json --diff $(ROOTDIR)/...

.PHONY: trim
trim: trim/go trim/go/test ## remove trailing blank lines from Go files

.PHONY: trim/go
trim/go: files ## remove trailing blank lines from non-test Go files
	@$(call green, "Trimming trailing blank lines from Go files...")
	@if [ -f "$(ROOTDIR)/.gitfiles" ]; then \
		grep -e "\.go$$" "$(ROOTDIR)/.gitfiles" | grep -v "_test\.go$$" \
		| xargs $(XARGS_NO_RUN_IF_EMPTY) -I {} -P"$(CORES)" \
		sed -z -i 's/\n\+$$/\n/' {}; \
	fi
	@$(call green, "Trim complete.")

.PHONY: trim/go/test
trim/go/test: files ## trim trailing blank lines in test Go files
	@$(call green, "Trimming trailing blank lines from Go test files...")
	@if [ -f "$(ROOTDIR)/.gitfiles" ]; then \
		grep -e "_test\.go$$" "$(ROOTDIR)/.gitfiles" \
		| xargs $(XARGS_NO_RUN_IF_EMPTY) -I {} -P"$(CORES)" \
		sed -z -i 's/\n\+$$/\n/' {}; \
	fi
	@$(call green, "Trim complete.")
