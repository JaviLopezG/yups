BINARY := yups

# Distro(s) for the integration tests: ubuntu | fedora | arch | opensuse,
# a comma separated combination, or all (default: all).
DISTRO ?= all

# Pretty printer for the go test output (colours, readable test names,
# live grouped view of parallel tests; see scripts/pretty-tests.sh).
PRETTIFY := ./scripts/pretty-tests.sh

# Pipefail so that a failing go test is not masked by the pretty printer
# succeeding.
SHELL       := /bin/bash
.SHELLFLAGS := -o pipefail -c

.PHONY: build test test-unit vet lint test-integration clean

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

# Aggregated linter. Skipped with a warning when it is not installed:
#   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
	golangci-lint run; \
	else \
	echo 'golangci-lint not found; skipping (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)'; \
	fi

test: vet lint test-unit

test-unit:
	@go test -v ./... | $(PRETTIFY)

# Builds the binary for linux and runs every scenario inside containers of
# the requested distro(s) (requires a running docker daemon).
#
#   make test-integration                     # every distro (default)
#   make test-integration DISTRO=fedora       # one specific distro
#   make test-integration DISTRO=fedora,arch  # several distros
test-integration:
	@YUPS_TEST_DISTRO=$(DISTRO) go test -tags integration -parallel 8 ./integration/ -count=1 -v | $(PRETTIFY)

clean:
	rm -f $(BINARY)
