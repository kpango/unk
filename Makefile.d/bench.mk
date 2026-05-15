
BENCH_FILTER   ?= .
BENCH_BASE_DIR  = $(TEST_RESULT_DIR)/bench

.PHONY: bench
bench: ## run all benchmarks (BENCH_FILTER=. by default)
	@$(call green, "Running benchmarks (filter: $(BENCH_FILTER))...")
	$(call go-bench,$(BENCH_FILTER),$(BENCH_BASE_DIR)/all.bin,$(ROOTDIR)/...)
	@$(call green, "Benchmarks complete.")

.PHONY: bench/cmd
bench/cmd: ## run benchmarks for cmd/
	@$(call green, "Running cmd benchmarks...")
	$(call go-bench,$(BENCH_FILTER),$(BENCH_BASE_DIR)/cmd.bin,$(ROOTDIR)/cmd/...)
	@$(call green, "cmd benchmarks complete.")

.PHONY: bench/internal
bench/internal: ## run benchmarks for internal/
	@$(call green, "Running internal benchmarks...")
	$(call go-bench,$(BENCH_FILTER),$(BENCH_BASE_DIR)/internal.bin,$(ROOTDIR)/internal/...)
	@$(call green, "internal benchmarks complete.")

.PHONY: bench/compare
bench/compare: benchstat/install ## compare benchmark results with benchstat
	@$(call green, "Comparing benchmarks...")
	@if ls $(BENCH_BASE_DIR)/*.out 2>/dev/null | wc -l | grep -qv "^[01]$$"; then \
		$(GOBIN)/benchstat $(BENCH_BASE_DIR)/*.out; \
	else \
		echo "Need at least two *.out files in $(BENCH_BASE_DIR)/ to compare"; \
		exit 1; \
	fi

.PHONY: bench/clean
bench/clean: ## remove benchmark result files
	rm -rf $(BENCH_BASE_DIR)

.PHONY: benchstat/install
benchstat/install: $(GOBIN)/benchstat ## install benchstat

$(GOBIN)/benchstat:
	@$(call green, "installing benchstat...")
	$(GOENV) go install golang.org/x/perf/cmd/benchstat@latest
