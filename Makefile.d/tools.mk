
.PHONY: golangci-lint/install
golangci-lint/install: $(BINDIR)/golangci-lint ## install golangci-lint

$(BINDIR)/golangci-lint:
	@$(call green, "installing golangci-lint $(GOLANGCILINT_VERSION)...")
	curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(BINDIR) $(GOLANGCILINT_VERSION)

.PHONY: goimports/install
goimports/install: $(GOBIN)/goimports ## install goimports

$(GOBIN)/goimports:
	@$(call green, "installing goimports...")
	$(GOENV) go install golang.org/x/tools/cmd/goimports@latest

.PHONY: strictgoimports/install
strictgoimports/install: $(GOBIN)/strictgoimports ## install strictgoimports

$(GOBIN)/strictgoimports:
	@$(call green, "installing strictgoimports...")
	$(GOENV) go install github.com/mzz2017/strictgoimports@latest

.PHONY: gofumpt/install
gofumpt/install: $(GOBIN)/gofumpt ## install gofumpt

$(GOBIN)/gofumpt:
	@$(call green, "installing gofumpt...")
	$(GOENV) go install mvdan.cc/gofumpt@latest

.PHONY: golines/install
golines/install: $(GOBIN)/golines ## install golines

$(GOBIN)/golines:
	@$(call green, "installing golines...")
	$(GOENV) go install github.com/segmentio/golines@latest

.PHONY: crlfmt/install
crlfmt/install: $(GOBIN)/crlfmt ## install crlfmt

$(GOBIN)/crlfmt:
	@$(call green, "installing crlfmt...")
	$(GOENV) go install github.com/cockroachdb/crlfmt@latest

.PHONY: tparse/install
tparse/install: $(GOBIN)/tparse ## install tparse

$(GOBIN)/tparse:
	@$(call green, "installing tparse...")
	$(GOENV) go install github.com/mfridman/tparse@latest

.PHONY: gotestfmt/install
gotestfmt/install: $(GOBIN)/gotestfmt ## install gotestfmt

$(GOBIN)/gotestfmt:
	@$(call green, "installing gotestfmt...")
	$(GOENV) go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@latest

.PHONY: gopls/install
gopls/install: $(GOBIN)/gopls ## install gopls

$(GOBIN)/gopls:
	@$(call green, "installing gopls...")
	$(GOENV) go install golang.org/x/tools/gopls@latest

.PHONY: staticcheck/install
staticcheck/install: $(GOBIN)/staticcheck ## install staticcheck

$(GOBIN)/staticcheck:
	@$(call green, "installing staticcheck...")
	$(GOENV) go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: tools/install
tools/install: ## install all development tools
tools/install: \
	golangci-lint/install \
	goimports/install \
	strictgoimports/install \
	gofumpt/install \
	golines/install \
	crlfmt/install \
	tparse/install \
	gotestfmt/install \
	gopls/install \
	staticcheck/install
	@$(call green, "all tools installed.")
