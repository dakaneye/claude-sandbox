.PHONY: build test test-e2e lint install clean

BINARY := claude-sandbox
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/claude-sandbox

test:
	go test -v -race ./...

test-e2e: install
	bash test/e2e/workflow_test.sh

lint:
	golangci-lint run ./...

install: build
	cp bin/$(BINARY) ~/go/bin/

clean:
	rm -rf bin/
