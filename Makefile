BINARY := yups

# Distro(s) for the integration tests: ubuntu | fedora | arch | opensuse,
# a comma separated combination, or all (default: ubuntu).
DISTRO ?= ubuntu

.PHONY: build test test-unit test-integration test-integration-all vet clean

build:
	go build -o $(BINARY) .

vet:
	go vet ./...

test: test-unit

test-unit:
	go test ./...

# Builds the binary for linux and runs every scenario inside containers of
# the requested distro(s) (requires a running docker daemon).
#
#   make test-integration                     # ubuntu only (default)
#   make test-integration DISTRO=fedora       # one specific distro
#   make test-integration DISTRO=fedora,arch  # several distros
test-integration:
	YUPS_TEST_DISTRO=$(DISTRO) go test -tags integration ./integration/ -count=1 -v

test-integration-all:
	YUPS_TEST_DISTRO=all go test -tags integration ./integration/ -count=1 -v

clean:
	rm -f $(BINARY)

