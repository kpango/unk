
.PHONY: test
test: ## run all tests
	@$(call green, "Running tests...")
	$(call go-test,$(ROOTDIR)/...)
	@$(call green, "Tests complete.")

.PHONY: test/cmd
test/cmd: ## run tests for cmd/
	@$(call green, "Running cmd tests...")
	$(call go-test,$(ROOTDIR)/cmd/...)
	@$(call green, "cmd tests complete.")

.PHONY: test/internal
test/internal: ## run tests for internal/
	@$(call green, "Running internal tests...")
	$(call go-test,$(ROOTDIR)/internal/...)
	@$(call green, "internal tests complete.")

.PHONY: test/verbose
test/verbose: ## run all tests with verbose output
	@$(call green, "Running tests (verbose)...")
	$(GOENV) go test $(GO_TEST_FLAGS) -v $(ROOTDIR)/...
	@$(call green, "Tests complete.")

.PHONY: test/tparse
test/tparse: tparse/install ## run all tests with tparse output
	@$(call green, "Running tests (tparse)...")
	$(call go-test-tparse,$(ROOTDIR)/...,test)
	@$(call green, "Tests complete.")

.PHONY: test/tparse/cmd
test/tparse/cmd: tparse/install ## run cmd tests with tparse output
	@$(call green, "Running cmd tests (tparse)...")
	$(call go-test-tparse,$(ROOTDIR)/cmd/...,test-cmd)
	@$(call green, "cmd tests complete.")

.PHONY: test/tparse/internal
test/tparse/internal: tparse/install ## run internal tests with tparse output
	@$(call green, "Running internal tests (tparse)...")
	$(call go-test-tparse,$(ROOTDIR)/internal/...,test-internal)
	@$(call green, "internal tests complete.")

.PHONY: test/gotestfmt
test/gotestfmt: gotestfmt/install ## run all tests with gotestfmt output
	@$(call green, "Running tests (gotestfmt)...")
	$(call go-test-gotestfmt,$(ROOTDIR)/...,test,-showteststatus)
	@$(call green, "Tests complete.")

.PHONY: test/gotestfmt/cmd
test/gotestfmt/cmd: gotestfmt/install ## run cmd tests with gotestfmt output
	@$(call green, "Running cmd tests (gotestfmt)...")
	$(call go-test-gotestfmt,$(ROOTDIR)/cmd/...,test-cmd,-showteststatus -hide="all")
	@$(call green, "cmd tests complete.")

.PHONY: test/gotestfmt/internal
test/gotestfmt/internal: gotestfmt/install ## run internal tests with gotestfmt output
	@$(call green, "Running internal tests (gotestfmt)...")
	$(call go-test-gotestfmt,$(ROOTDIR)/internal/...,test-internal,-showteststatus -hide="all")
	@$(call green, "internal tests complete.")

.PHONY: coverage
coverage: ## generate test coverage HTML report
	@$(call green, "Generating coverage report...")
	@mkdir -p $(TEST_RESULT_DIR)
	$(GOENV) go test $(GO_TEST_FLAGS) -covermode=atomic -coverprofile=$(TEST_RESULT_DIR)/coverage.out $(ROOTDIR)/...
	$(GOENV) go tool cover -html=$(TEST_RESULT_DIR)/coverage.out -o $(TEST_RESULT_DIR)/coverage.html
	@$(call green, "Coverage report: $(TEST_RESULT_DIR)/coverage.html")
