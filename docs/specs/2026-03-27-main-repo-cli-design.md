# Main-Repo-Centric CLI Refactor

## Overview

Refactor claude-sandbox CLI so all commands run from the main repository, eliminating the need to `cd` into worktrees. Work still happens in isolated worktrees, but the CLI manages them transparently.

## Motivation

Current workflow requires:
```bash
claude-sandbox init .
cd ../project-sandbox-abc123    # friction point
# create spec...
claude-sandbox run --spec ./spec.md
```

New workflow:
```bash
claude-sandbox spec             # creates worktree, launches Claude for SPEC.md + PLAN.md
claude-sandbox execute          # runs in worktree, returns
claude-sandbox ship             # creates PR, cleans up
```

## CLI Commands

### `spec`

Creates a new sandbox session and launches Claude for spec-driven planning.

```bash
claude-sandbox spec [--name NAME]
```

**Behavior:**
1. Validate cwd is a git repository
2. Create worktree with branch `sandbox/<date>-<hash>`
3. Create session in `.claude-sandbox/sessions/<id>.json` with status `speccing`
4. Set as active session
5. Launch interactive Claude session in the worktree with initial prompt:
   - "Use /superpowers:brainstorming to create SPEC.md, then /superpowers:writing-plans to create PLAN.md"
   - User collaborates with Claude to refine spec and plan
6. When Claude exits:
   - If `PLAN.md` exists → set status to `ready`
   - If not → set status to `failed` with error "Spec incomplete"
7. Return to main repo

**Flags:**
- `--name NAME`: Optional friendly name for the session (creates symlink)

### `execute`

Executes the plan in a sandboxed container.

```bash
claude-sandbox execute [--session ID]
```

**Behavior:**
1. Resolve session (see Session Resolution)
2. Check `COMPLETION.md` status if exists:
   - **SUCCESS**: Print message, exit (nothing to do)
   - **FAILED/BLOCKED**: Continue execution (idempotent)
   - **Not exists**: Start fresh execution
3. Launch container with worktree mounted
4. Claude executes `PLAN.md` with quality gates
5. Write `COMPLETION.md` on completion

**Flags:**
- `--session ID`: Explicit session selection (ID or name)

### `status`

Shows session status with AI-powered progress analysis.

```bash
claude-sandbox status [--session ID]
```

**Output:**
- Session ID, name, branch
- Status (running/success/failed/blocked)
- Elapsed time
- If running: AI analysis of log tail (current phase, progress estimate)

### `stop`

Stops a running session.

```bash
claude-sandbox stop [--session ID]
```

**Behavior:**
1. Resolve session
2. Stop Docker container
3. Mark session as failed with "stopped by user"

### `ship`

Creates PR from completed work.

```bash
claude-sandbox ship [--session ID] [--keep-worktree]
```

**Behavior:**
1. Resolve session
2. Validate `COMPLETION.md` shows SUCCESS status
3. Prompt user to review `COMPLETION.md`
4. Launch Claude with `/create-pr` skill
5. Unless `--keep-worktree`: prompt to clean up worktree

### `clean`

Removes sandbox sessions and worktrees.

```bash
claude-sandbox clean [--session ID | --all]
```

**Behavior:**
- No flags: Interactive picker to select session(s)
- `--session ID`: Remove specific session
- `--all`: Remove all sessions (with confirmation)

Removes both worktree and `.claude-sandbox/sessions/<id>.json`.

## State Management

### Directory Structure

```
<repo>/
  .claude-sandbox/
    sessions/
      abc123.json           # session state
      feature-x.json        # symlink to abc123.json (if --name used)
    active                  # contains ID of most recent session
```

Add to `.gitignore`:
```
.claude-sandbox/
```

### Session Schema

```json
{
  "id": "abc123",
  "name": "feature-x",
  "worktree_path": "/absolute/path/to/project-sandbox-abc123",
  "branch": "sandbox/2026-03-27-abc123",
  "status": "running",
  "log_path": "/home/user/.claude/sandbox-sessions/abc123.log",
  "created_at": "2026-03-27T10:00:00Z",
  "completed_at": null,
  "error": null
}
```

**Status values and transitions:**
- `speccing`: Interactive Claude session for SPEC.md/PLAN.md
  - → `ready` when Claude exits and PLAN.md exists
  - → `failed` when Claude exits without PLAN.md
- `ready`: Spec complete, awaiting execution
  - → `running` when `execute` starts
- `running`: Container executing the plan
  - → `success` when all quality gates pass
  - → `failed` on error
  - → `blocked` when quality gates cannot be satisfied after retries
- `success`: Complete, ready to ship
- `failed`: Execution failed (can retry with `execute`)
- `blocked`: Quality gates unsatisfiable (can retry with `execute`)

### Session Resolution

When `--session` is not provided:

1. If exactly one session exists → use it
2. If multiple sessions exist → interactive picker showing:
   - Session ID/name
   - Status
   - Created time
3. If no sessions exist → error with guidance

## Standard File Locations

All paths relative to worktree root:

| File | Purpose |
|------|---------|
| `SPEC.md` | Feature specification (from brainstorming) |
| `PLAN.md` | Implementation plan (from writing-plans) |
| `COMPLETION.md` | Execution report (success/failure) |
| `session.json` | Removed — state now in main repo |

## Package Structure

### New: `internal/state/`

Manages `.claude-sandbox/` directory:

```go
// EnsureDir creates .claude-sandbox/sessions/ if needed
func EnsureDir(repoPath string) error

// Create creates a new session and sets it as active
func Create(repoPath string, opts CreateOptions) (*Session, error)

// Get loads a session by ID or name
func Get(repoPath, idOrName string) (*Session, error)

// GetActive returns the active session, or prompts picker if multiple
func GetActive(repoPath string) (*Session, error)

// List returns all sessions
func List(repoPath string) ([]*Session, error)

// SetActive updates the active pointer
func SetActive(repoPath, id string) error

// Remove deletes session state and optionally the worktree
func Remove(repoPath, id string, removeWorktree bool) error
```

### Modified: `internal/session/`

Remove or deprecate — functionality moves to `internal/state/`.

### Modified: `internal/cli/`

| File | Changes |
|------|---------|
| `init.go` | Rename to `spec.go`, rewrite for new flow |
| `run.go` | Rename to `execute.go`, use state resolution |
| `status.go` | Use state resolution instead of requireWorktree |
| `stop.go` | Use state resolution |
| `ship.go` | Use state resolution |
| `clean.go` | Rewrite to use state package |
| `logs.go` | Remove (status covers this) |
| `helpers.go` | Remove `requireWorktree()`, add `resolveSession()` |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| `spec` in non-git directory | Error: "Not a git repository" |
| `execute` with no sessions | Error: "No sessions. Run 'claude-sandbox spec' first" |
| `execute` on SUCCESS session | Message: "Already completed successfully" |
| `execute` on FAILED/BLOCKED session | Resume execution (idempotent) |
| `ship` on non-SUCCESS session | Error: "Cannot ship incomplete work" |
| Worktree deleted externally | Mark session stale, remove from state on next access |
| Multiple sessions, no --session | Interactive picker |

## Migration

This is a breaking change. The `run` command (now `execute`) no longer works from within a worktree. Users must run all commands from the main repository.

**Migration path:**
1. If work exists in old worktrees, complete it manually
2. Run `claude-sandbox clean --all` to remove old sessions
3. Start fresh with `claude-sandbox spec`

## Future Considerations

Not in scope:
- `sync` command to pull main repo changes into worktree
- Parallel execution of multiple sessions
- Remote/cloud execution
