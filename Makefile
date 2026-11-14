

default: verify

build:
	go build .

lint:
	go fmt ./...
	go fix ./...
	golangci-lint run ./...
	gosec ./...
	go vet ./...

test:
	go test -race ./...

run-examples:
	.github/scripts/run-examples.sh

verify: lint test build
