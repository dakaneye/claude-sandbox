# Image Build CLI Integration

Add container image build functionality directly into the claude-sandbox CLI.

## Problem

Currently, building the sandbox container image requires:
1. Having the repo checked out
2. Running `cd container && ./build.sh --load`
3. Having apko installed separately

This creates friction for first-time users and makes the CLI less self-contained.

## Solution

Embed the build configs in the Go binary and add:
1. `claude-sandbox build` - explicit build command
2. Auto-build in `execute` when image is missing

## Requirements

- **Require apko**: Fail with install instructions if not found
- **Always prebake**: No option to skip prebaking Claude Code
- **Local only**: Build for current architecture, no registry push
- **Auto-build only if missing**: No version checks or freshness validation
- **Fail immediately on error**: No retries or fallbacks

## Architecture

### File Structure

```
internal/container/
  build.go              # Build(), checkApko(), checkDocker()
  build_test.go         # Unit and integration tests
  configs/
    apko.yaml           # Moved from container/claude-sandbox.apko.yaml
    prebake.dockerfile  # Moved from container/Dockerfile.prebake
  embed.go              # //go:embed directives
  container.go          # Existing: Run(), Stop(), ImageExists()

internal/cli/
  build.go              # claude-sandbox build command
  execute.go            # Modified: auto-build if image missing
```

### Removed

The `container/` directory is deleted entirely:
- `build.sh` → replaced by Go code
- `claude-sandbox.apko.yaml` → moved to `internal/container/configs/apko.yaml`
- `Dockerfile.prebake` → moved to `internal/container/configs/prebake.dockerfile`

### Updated

- `CLAUDE.md` - remove reference to `cd container && ./build.sh --load`
- `.gitignore` - remove `container/*.tar` and `container/sbom-*.spdx.json` patterns

## Build Command

```
claude-sandbox build [flags]

Flags:
  --force    Rebuild even if image exists
```

### Behavior

1. Check Docker is running → fail with message if not
2. Check apko is installed → fail with install instructions if not
3. Check if image exists (unless `--force`) → print message and exit 0 if exists
4. Detect current architecture (amd64 or arm64 from `runtime.GOARCH`)
5. Write embedded configs to temp directory
6. Run `apko build apko.yaml claude-sandbox:base <tempfile>.tar`
7. Run `docker load < <tempfile>.tar` (loads as `claude-sandbox:base-<arch>`)
8. Run `docker build -t claude-sandbox:latest --build-arg BASE_IMAGE=claude-sandbox:base-<arch> -f prebake.dockerfile .`
9. Clean up temp files
10. Print "Image built: claude-sandbox:latest"

## Auto-build in Execute

```go
// In runExecute(), after session resolution:
if !container.ImageExists(container.DefaultImage) {
    cmd.Println("Container image not found. Building...")
    if err := container.Build(cmd.OutOrStdout()); err != nil {
        return fmt.Errorf("build image: %w", err)
    }
    cmd.Println()
}
```

## Embedded Configs

```go
// internal/container/embed.go
package container

import _ "embed"

//go:embed configs/apko.yaml
var apkoConfig []byte

//go:embed configs/prebake.dockerfile
var prebakeDockerfile []byte
```

Single source of truth - configs live only in `internal/container/configs/`.

## Error Handling

| Scenario | Error Message |
|----------|---------------|
| Docker not running | "Docker is not running. Start Docker and try again." |
| apko not found | "apko not found. Install with: brew install apko (macOS) or go install chainguard.dev/apko@latest" |
| apko build fails | "apko build failed: \<stderr output\>" |
| docker load fails | "docker load failed: \<stderr output\>" |
| docker build fails | "docker build failed: \<stderr output\>" |
| Image already exists | "Image claude-sandbox:latest already exists. Use --force to rebuild." (exit 0) |

## Testing

### Unit Tests (`internal/container/build_test.go`)

- `TestCheckApko` - verify apko detection logic
- `TestCheckDocker` - verify docker detection logic
- `TestWriteConfigs` - verify temp file creation
- `TestBuildArgs` - verify apko/docker command construction

### Integration Tests

- Build tag: `//go:build integration`
- Test actual build when Docker and apko are available
- Run with: `go test -tags=integration ./...`

### Quality Gates

Before marking work complete:
1. `go test ./...` - unit tests pass
2. `go test -tags=integration ./...` - integration tests pass (when Docker/apko available)
3. `golangci-lint run ./...` - no lint errors
4. `go build ./...` - builds successfully
5. `/review-code` - grade A
