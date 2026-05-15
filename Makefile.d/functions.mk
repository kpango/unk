
# --- Color Definitions ---

red    = printf "\x1b[31m\#\# %s\x1b[0m\n" $1
green  = printf "\x1b[32m\#\# %s\x1b[0m\n" $1
yellow = printf "\x1b[33m\#\# %s\x1b[0m\n" $1
blue   = printf "\x1b[34m\#\# %s\x1b[0m\n" $1
pink   = printf "\x1b[35m\#\# %s\x1b[0m\n" $1
cyan   = printf "\x1b[36m\#\# %s\x1b[0m\n" $1

# --- Environment Variable Bundles ---

GOENV = \
	GOPKG=$(GOPKG) \
	GOARCH=$(GOARCH) \
	GOOS=$(GOOS) \
	GO111MODULE=on \
	GO_VERSION=$(GO_VERSION)

# --- Go Lint ---

define go-lint
	$(BINDIR)/golangci-lint run --config $(ROOTDIR)/.golangci.json --fix $(ROOTDIR)/...
endef

define go-lint-nofix
	$(BINDIR)/golangci-lint run --config $(ROOTDIR)/.golangci.json $(ROOTDIR)/...
endef

# --- Go Vet ---

define go-vet
	$(GOENV) go vet $(ROOTDIR)/...
endef

# --- Go Test ---

define go-test
	$(GOENV) go test $(GO_TEST_FLAGS) $1
endef

define go-test-json
	set -euo pipefail; \
	mkdir -p $(TEST_RESULT_DIR); \
	rm -f "$(TEST_RESULT_DIR)/$$(echo $2 | sed -e 's%/%-%g')-result.json"; \
	$(GOENV) go test $(GO_TEST_FLAGS) -json $1 \
		| tee "$(TEST_RESULT_DIR)/$$(echo $2 | sed -e 's%/%-%g')-result.json"
endef

define go-test-tparse
	set -euo pipefail; \
	mkdir -p $(TEST_RESULT_DIR); \
	rm -f "$(TEST_RESULT_DIR)/$$(echo $2 | sed -e 's%/%-%g')-result.json"; \
	$(GOENV) go test $(GO_TEST_FLAGS) -json $1 \
		| tee "$(TEST_RESULT_DIR)/$$(echo $2 | sed -e 's%/%-%g')-result.json" \
		| tparse -pass -notests
endef

define go-test-gotestfmt
	set -euo pipefail; \
	mkdir -p $(TEST_RESULT_DIR); \
	rm -f "$(TEST_RESULT_DIR)/$$(echo $2 | sed -e 's%/%-%g')-result.json"; \
	$(GOENV) go test $(GO_TEST_FLAGS) -json $1 \
		| tee "$(TEST_RESULT_DIR)/$$(echo $2 | sed -e 's%/%-%g')-result.json" \
		| gotestfmt $3
endef

# --- Go Benchmark ---
# $1: bench filter (e.g. "." for all)
# $2: output path prefix (e.g. $(TEST_RESULT_DIR)/bench/foo.bin)
# $3: package path

define go-bench
	mkdir -p $(dir $2)
	$(GOENV) go test \
		-mod=readonly \
		-count=3 \
		-timeout=1h \
		-bench=$1 \
		-benchmem \
		-benchtime=3s \
		-cpuprofile $(patsubst %.bin,%.cpu.out,$2) \
		-memprofile $(patsubst %.bin,%.mem.out,$2) \
		$3
endef
