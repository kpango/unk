
SHELL = bash
.DEFAULT_GOAL := help

ORG     ?= kpango
NAME     = unk
REPO     = $(ORG)/$(NAME)
GOPKG    = github.com/$(REPO)
DATETIME = $(eval DATETIME := $(shell date -u +%Y/%m/%d_%H:%M:%S%z))$(DATETIME)

XARGS_NO_RUN_IF_EMPTY := $(eval XARGS_NO_RUN_IF_EMPTY := $(shell xargs --version 2>/dev/null | head -1 | grep -qi gnu && echo -r))$(XARGS_NO_RUN_IF_EMPTY)

GOTEST_TIMEOUT  = 30m
GO_CLEAN_DEPS  := true

GOPATH   ?= $(eval GOPATH   := $(shell go env GOPATH   2>/dev/null))$(GOPATH)
GOARCH   ?= $(eval GOARCH   := $(shell go env GOARCH   2>/dev/null))$(GOARCH)
GOBIN    ?= $(eval GOBIN    := $(or $(shell go env GOBIN 2>/dev/null),$(GOPATH)/bin))$(GOBIN)
GOCACHE  ?= $(eval GOCACHE  := $(shell go env GOCACHE  2>/dev/null))$(GOCACHE)
GOOS     ?= $(eval GOOS     := $(shell go env GOOS     2>/dev/null))$(GOOS)
GO_VERSION ?= $(eval GO_VERSION := $(shell go env GOVERSION 2>/dev/null | sed 's/go//'))$(GO_VERSION)

GOLANGCILINT_VERSION ?= latest

UNAME := $(eval UNAME := $(shell uname -s))$(UNAME)
OS    := $(eval OS    := $(shell echo $(UNAME) | tr '[:upper:]' '[:lower:]'))$(OS)
ARCH  := $(eval ARCH  := $(shell uname -m))$(ARCH)
PWD   := $(eval PWD   := $(shell pwd))$(PWD)

ifeq ($(UNAME),Linux)
CORES := $(eval CORES := $(shell nproc 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null))$(CORES)
else ifeq ($(UNAME),Darwin)
CORES := $(eval CORES := $(shell sysctl -n hw.ncpu 2>/dev/null || getconf _NPROCESSORS_ONLN 2>/dev/null))$(CORES)
else
CORES := 1
endif

GIT_COMMIT := $(eval GIT_COMMIT := $(shell git rev-list -1 HEAD 2>/dev/null || echo "unknown"))$(GIT_COMMIT)
ROOTDIR = $(eval ROOTDIR := $(or $(shell git rev-parse --show-toplevel 2>/dev/null),$(PWD)))$(ROOTDIR)

USR_LOCAL = /usr/local
BINDIR    = $(USR_LOCAL)/bin

TEMP_DIR := $(eval TEMP_DIR := $(shell mktemp -d))$(TEMP_DIR)

TEST_RESULT_DIR ?= $(ROOTDIR)/test-results

GO_BUILD_FLAGS     = -mod=readonly -trimpath
GO_TEST_BASE_FLAGS = -short -shuffle=on -race -mod=readonly -cover -timeout=$(GOTEST_TIMEOUT)
GO_TEST_FLAGS      = $(GO_TEST_BASE_FLAGS) -ldflags="-s -w"
GO_LDFLAGS         = -s -w

include Makefile.d/functions.mk
include Makefile.d/tools.mk
include Makefile.d/format.mk
include Makefile.d/lint.mk
include Makefile.d/test.mk
include Makefile.d/bench.mk

.PHONY: all
all: format lint test ## run format, lint, and test

.PHONY: help
help: ## show this help message
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_\/-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort

.PHONY: files
files: ## update .gitfiles with current tracked and untracked files
	@git ls-files --cached --others --exclude-standard | sort > $(ROOTDIR)/.gitfiles

.PHONY: build
build: ## build the unk binary
	@mkdir -p $(ROOTDIR)/bin
	$(GOENV) go build $(GO_BUILD_FLAGS) \
		-ldflags "$(GO_LDFLAGS) \
		-X 'main.Version=$(GIT_COMMIT)' \
		-X 'main.BuildTime=$(DATETIME)'" \
		-o $(ROOTDIR)/$(NAME) \
		$(ROOTDIR)/cmd/$(NAME)/main.go

.PHONY: clean
clean: ## remove build artifacts and test results
	rm -rf $(ROOTDIR)/bin $(TEST_RESULT_DIR)

.PHONY: deps
deps: ## tidy and verify go modules
	go mod tidy
	go mod verify
