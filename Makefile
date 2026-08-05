.PHONY: serve lint test test-e2e image

LINKER_FLAGS = '-s -X main.version=${PORTAL_VERSION}'

lint:
	golangci-lint run --timeout 5m -E misspell ./...

build:
	go build -ldflags=${LINKER_FLAGS} -o portal ./cmd/portal/

build-production:
	CGO_ENABLED=0 go build -ldflags=${LINKER_FLAGS} -o portal ./cmd/portal/

image:
	docker build --build-arg version=${PORTAL_VERSION} --tag rendezvous:latest .

serve: image
	docker run -dp 8080:8080 rendezvous:latest

test:
	go test -ldflags=${LINKER_FLAGS} -v -race -covermode=atomic -coverprofile=coverage.out -failfast -short ./...

test-e2e: image
	go test -ldflags=${LINKER_FLAGS} -v -race -covermode=atomic -coverprofile=coverage.out -failfast ./...
