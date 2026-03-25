.PHONY: build test lint install clean

BINARY := claude-sandbox
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/claude-sandbox

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

install: build
	cp bin/$(BINARY) ~/go/bin/

clean:
	rm -rf bin/
