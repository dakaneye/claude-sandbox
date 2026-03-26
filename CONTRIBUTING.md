# Contributing

## Development

```bash
git clone https://github.com/dakaneye/claude-sandbox.git
cd claude-sandbox
make build
```

## Commands

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make install    # Install to ~/go/bin
make clean      # Remove build artifacts
```

## Container Image

```bash
cd container && ./build.sh --load
```

Requires [apko](https://github.com/chainguard-dev/apko) for building the base image.

## Before Submitting

1. `make build` passes
2. `make lint` passes (or `golangci-lint run ./...`)
3. `go test -race ./...` passes
4. `go mod tidy` produces no changes
5. New functionality has tests

## Pull Requests

- Keep changes focused
- Update tests for new functionality
- Follow existing code style
- Run `gofmt` and `goimports` before committing

## Pre-commit Hooks

Install pre-commit hooks to catch issues before committing:

```bash
brew install pre-commit
pre-commit install
```
