# make is a local development/testing helper. The production relay image is
# built by CI with its own version, so the docker targets below only ever build
# a throwaway local image. Test targets therefore use a fixed TEST_VERSION
# instead of requiring PORTAL_VERSION.

.PHONY: serve lint test test-e2e image build build-production

# Version injected into release binaries via -ldflags. Required for
# build / build-production.
LINKER_FLAGS = '-s -X main.version=${PORTAL_VERSION}'

# Fixed version for local test images and test binaries. Overridable, e.g.
#   make test-e2e TEST_VERSION=v1.2.3
TEST_VERSION = v0.0.0
TEST_LINKER_FLAGS = '-s -X main.version=${TEST_VERSION}'

lint:
	golangci-lint run --timeout 5m -E misspell ./...

build:
	go build -ldflags=${LINKER_FLAGS} -o portal ./cmd/portal/

build-production:
	CGO_ENABLED=0 go build -ldflags=${LINKER_FLAGS} -o portal ./cmd/portal/

image:
	docker build --build-arg version=${TEST_VERSION} --tag rendezvous:latest .

serve: image
	docker run -dp 8080:8080 rendezvous:latest

test:
	go test -ldflags=${TEST_LINKER_FLAGS} -v -race -covermode=atomic -coverprofile=coverage.out -failfast -short ./...

test-e2e: image
	go test -ldflags=${TEST_LINKER_FLAGS} -v -race -covermode=atomic -coverprofile=coverage.out -failfast ./...
