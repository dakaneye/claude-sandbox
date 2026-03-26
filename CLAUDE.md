# Claude Development Guidelines for claude-sandbox

## Project Overview

CLI tool for running Claude Code in sandboxed Docker containers with git worktree isolation.

## Architecture

```
cmd/claude-sandbox/main.go    Entry point
internal/
  cli/                        Cobra commands (init, run, ship, status, logs, stop, clean)
  cli/helpers.go              Shared CLI utilities (requireWorktree, promptYesNo)
  container/                  Docker container management
  session/                    Session state (JSON persisted to worktree)
  worktree/                   Git worktree operations
  id/                         Shared ID generation utilities
container/                    Container image build (apko + hooks)
```

## Key Patterns

### CLI Commands
- Use `newXxxCommand()` factory pattern returning `*cobra.Command`
- Use `requireWorktree()` helper for commands that need worktree context
- Use `promptYesNo()` helper for confirmations
- Output via `cmd.Println()` / `cmd.PrintErrf()`, not `fmt.Print`

### Error Handling
- Wrap with context: `fmt.Errorf("action: %w", err)`
- State action directly, no "failed to" prefix: `"open file: %w"` not `"failed to open file: %w"`

### Container Naming
- Deterministic names from worktree path: `ContainerName(worktreePath)` → `claude-sandbox-<hash12>`
- Enables stopping containers by worktree path

### Session State
- Persisted to `session.json` in worktree root
- Logs stored at `~/.claude/sandbox-sessions/<session-id>.log`

## Commands

```bash
# Build
make build

# Test
go test ./...

# Install locally
make install

# Build container image (requires apko)
cd container && ./build.sh --load
```

## Testing Conventions

- Test files use `setupTestRepo()` or `setupTestRepoForCLI()` helpers
- Helpers disable commit signing: `{"git", "config", "commit.gpgsign", "false"}`
- Use `t.TempDir()` for test directories (auto-cleanup)
- Resolve symlinks with `filepath.EvalSymlinks()` for macOS compatibility

## Quality Gates

Before committing, all gates must pass:

```bash
# 1. Build
make build

# 2. Lint
golangci-lint run ./...

# 3. Tests
go test ./...

# 4. Tidy modules
go mod tidy && git diff --exit-code go.mod go.sum

# 5. Container image build
cd container && ./build.sh --load
```

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- Docker - Container runtime
- apko - Container image build (in `container/`)
