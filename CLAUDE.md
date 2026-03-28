# Claude Development Guidelines for claude-sandbox

## Project Overview

CLI tool for running Claude Code in sandboxed Docker containers with git worktree isolation.

## Architecture

```
cmd/claude-sandbox/main.go    Entry point
internal/
  cli/                        Cobra commands (spec, execute, status, stop, ship, clean, list, build)
  cli/helpers.go              Shared CLI utilities (findRepoRoot, promptYesNo)
  cli/completion.go           COMPLETION.md status parsing (shared by execute + ship)
  cli/logsummary.go           Stream-json log parsing for status progress analysis
  container/                  Docker container management + embedded build configs
  state/                      Session state (JSON in .claude-sandbox/sessions/)
  worktree/                   Git worktree operations
  id/                         Shared ID generation utilities
```

## Key Patterns

### CLI Commands
- Use `newXxxCommand()` factory pattern returning `*cobra.Command`
- Use `findRepoRoot()` helper for commands that need repo context
- Use `state.ResolveSession()` for session lookup (ID, name, or interactive picker)
- Use `promptYesNo()` helper for confirmations
- Output via `cmd.Println()` / `cmd.PrintErrf()`, not `fmt.Print`

### Error Handling
- Wrap with context: `fmt.Errorf("action: %w", err)`
- State action directly, no "failed to" prefix: `"open file: %w"` not `"failed to open file: %w"`

### Container Naming
- Deterministic names from worktree path: `ContainerName(worktreePath)` → `claude-sandbox-<hash12>`
- Enables stopping containers by worktree path

### Session State
- Persisted to `.claude-sandbox/sessions/<id>.json` in main repo root
- Named sessions use symlinks: `<name>.json` → `<id>.json`
- Active session tracked in `.claude-sandbox/active`
- Logs stored at `~/.claude/sandbox-sessions/<session-id>.log`

### Superpowers Integration
When using superpowers skills (`/brainstorming`, `/writing-plans`), tell Claude to put
the plan in the **worktree root** as `PLAN.md`:

```
Put the plan in PLAN.md in the repository root.
```

By default, superpowers puts plans in `docs/specs/<date>-<name>.md`, but claude-sandbox
expects `PLAN.md` in the worktree root for the `execute` command.

## Commands

```bash
# Build
make build

# Test
go test ./...

# Install locally
make install

# Build container image (requires apko)
claude-sandbox build
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
claude-sandbox build

# 6. Code review (required - must be grade A)
/review-code
```

## Pre-commit Hooks

Pre-commit runs build, lint, and tidy automatically. Tests are excluded because
git worktree tests fail in pre-commit's stash context. Run `go test ./...`
manually before pushing, or rely on CI.

```bash
pre-commit install  # One-time setup
```

## Code Review

Run `/review-code` before every commit. Must achieve grade A.

Install the skill:
```bash
prpm install @dakaneye/dakaneye-review-code
```

See: https://prpm.dev/packages/@dakaneye/dakaneye-review-code

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- Docker - Container runtime
- apko - Container image build (embedded configs)
