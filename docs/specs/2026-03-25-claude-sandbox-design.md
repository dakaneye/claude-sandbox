# Claude Sandbox: Spec-Driven Autonomous Execution

## Overview

A sandboxed execution environment for Claude Code that enables autonomous implementation of specs while preventing external side effects. Combines container isolation, git worktrees, and hook-based gates to create an outcome-oriented workflow.

## Problem Statement

Running Claude Code with `--dangerously-skip-permissions` provides autonomy but risks:
1. Unintended system damage (filesystem/network)
2. Unreviewed external side effects (git push, PR creation, issue comments)
3. No quality gates before claiming work is "done"

The goal is to let Claude implement a spec without interruption while ensuring:
- Host system protection via container isolation
- External actions gated until human review
- Quality criteria enforced before completion

## Workflow

```
Phase 1: Init
  $ claude-sandbox init ~/dev/project
  → Creates git worktree for isolation

Phase 2: Plan & Spec (normal Claude session)
  → User creates spec in worktree

Phase 3: Autonomous Execution
  $ claude-sandbox run --spec ./path/to/spec
  → Claude implements in container with quality gates
  → COMPLETION.md written on success/blocked

Phase 4: Review & Ship
  $ claude-sandbox ship
  → User reviews, then PR created via /create-pr skill
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Host Machine                              │
│  ┌─────────────────┐    ┌─────────────────────────────────────┐ │
│  │  claude-sandbox │    │         Container (apko)            │ │
│  │      CLI        │───▶│  ┌─────────────────────────────────┐│ │
│  │                 │    │  │  Claude Code + hooks            ││ │
│  │  - init         │    │  │  --dangerously-skip-permissions ││ │
│  │  - run          │    │  │                                 ││ │
│  │  - ship         │    │  │  Quality gates enforced         ││ │
│  │  - status       │    │  │  External actions blocked       ││ │
│  │  - logs         │    │  └─────────────────────────────────┘│ │
│  │  - stop         │    │                                     │ │
│  │  - clean        │    │  Read-only mounts:                 │ │
│  └─────────────────┘    │  - ~/.claude/settings.json         │ │
│          │              │  - ~/.claude/hooks/                │ │
│          ▼              │  - ~/.claude/skills/               │ │
│  ┌─────────────────┐    │  - ~/.gitconfig, ~/.ssh/           │ │
│  │  Git Worktree   │◀───│                                     │ │
│  │  (mounted rw)   │    │  Read-write mounts:                │ │
│  └─────────────────┘    │  - worktree directory              │ │
│                         │  - Claude history volume           │ │
│                         └─────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## CLI Interface

### `claude-sandbox init <project-path>`

Creates a git worktree for isolated work.

**Arguments:**
- `project-path`: Path to the git repository

**Behavior:**
1. Validates path is a git repository
2. Creates worktree with branch name `sandbox/<date>-<hash>`
3. Returns worktree path

**Output:**
```
Creating worktree...
  Branch: sandbox/2026-03-25-abc123
  Path:   /path/to/project-sandbox-abc123

Worktree ready.
```

### `claude-sandbox run --spec <path>`

Launches Claude in container with spec.

**Flags:**
- `--spec <path>`: Path to spec file or directory (required)
- `--timeout <duration>`: Max execution time (default: 2h)
- `--retries <n>`: Max retry attempts per quality gate (default: 3)

**Behavior:**
1. Validates spec exists
2. Detects worktree (or uses current directory)
3. Launches container with mounts
4. Runs Claude with spec as prompt
5. Enforces quality gates
6. Writes COMPLETION.md
7. Fires claude-notify hook

**Output:**
```
Starting sandboxed Claude session...
  Spec:      ./docs/specs/feature/ (3 files)
  Worktree:  /path/to/project-sandbox-abc123
  Container: claude-sandbox:latest

Claude is working. You'll be notified on completion.
Session log: ~/.claude/sandbox-sessions/2026-03-25-abc123.log
```

### `claude-sandbox ship`

Creates PR after user review.

**Behavior:**
1. Validates COMPLETION.md exists with SUCCESS status
2. Prompts user to review COMPLETION.md
3. Prompts for confirmation
4. Invokes Claude with /create-pr skill (outside sandbox)
5. Optionally cleans up worktree

**Output:**
```
Review COMPLETION.md before shipping? [Y/n]
Ship this work? [y/N]
Launching Claude to create PR via /create-pr...
PR created: https://github.com/org/repo/pull/1234
Clean up worktree? [Y/n]
```

### `claude-sandbox status`

Shows status of running session.

### `claude-sandbox logs`

Tails the session log.

### `claude-sandbox stop`

Cancels running session.

### `claude-sandbox clean`

Removes stale worktrees.

## Quality Gates

All gates must pass before COMPLETION.md can be written with SUCCESS status.

### Gate Pipeline

```
Build → Lint → Test → Security → Spec Coverage → Commit Hygiene → /review-code
```

### Gate Detection

| Gate | Detection |
|------|-----------|
| Build | `package.json` → `npm run build`, `go.mod` → `go build ./...`, `Makefile` → `make` |
| Lint | `eslint.config.*` → `npm run lint`, `go.mod` → `golangci-lint run` |
| Test | `*_test.go` → `go test ./...`, `*.test.js` → `npm test` |
| Security | `package.json` → `npm audit`, `go.mod` → `govulncheck ./...` |
| Spec coverage | Claude verifies all spec items addressed |
| Commit hygiene | Atomic commits with conventional messages |
| Code review | `/review-code` skill must return grade A |

### Override via `.claude-sandbox.yaml`

```yaml
gates:
  build: "make build"
  lint: "make lint"
  test: "make test-unit && make test-integration"
  security: "make security-scan"
```

### Retry Behavior

- Max 3 retries per failing gate
- If still failing after retries, write COMPLETION.md with BLOCKED status
- Include diagnosis and what was attempted

## External Action Gates

### Blocked Actions

**Bash commands:**
- `git push` (all variants)
- `gh pr create`, `gh pr merge`, `gh pr comment`
- `gh issue create`, `gh issue comment`
- `curl -X POST/PUT/PATCH/DELETE`, `curl --data`, `curl -d`
- `wget --post-data`

**MCP tools:**
- `mcp__linear__createIssue`
- `mcp__linear__updateIssue`
- `mcp__linear__createComment`
- `mcp__linear__addLabel`
- `mcp__linear__removeLabel`
- `mcp__linear__archiveIssue`

### Allowed Actions

**Bash commands:**
- `git commit` (local)
- `gh pr view`, `gh issue view`, `gh api` (reads)
- `curl` GET requests (default)
- `npm install`, `go get`, `pip install`
- WebSearch tool

**MCP tools:**
- `mcp__linear__getIssue`, `mcp__linear__listIssues`, `mcp__linear__search*`
- All Kora/SQLite operations (local DB)

### Hook Implementation

The `sandbox-guard.sh` hook:
1. Checks `$TOOL_NAME`
2. If `Bash`: pattern match against blocklists
3. If `mcp__*`: check against MCP blocklist
4. Exit 0 to allow, exit 2 to block

## COMPLETION.md Format

```markdown
# Completion Report

## Status: SUCCESS | BLOCKED | PARTIAL

**Spec:** ./docs/specs/feature/
**Branch:** sandbox/2026-03-25-abc123
**Duration:** 47 minutes
**Session log:** ~/.claude/sandbox-sessions/2026-03-25-abc123.log

## Summary

One-line description of what was accomplished.

## Changes Made

- Bullet list of changes

## Files Changed

<git diff --stat output>

## Quality Gates

| Gate | Status | Notes |
|------|--------|-------|
| Build | PASS/FAIL | |
| Lint | PASS/FAIL | |
| Tests | PASS/FAIL | X passed, Y failed |
| Security | PASS/FAIL | |
| Spec coverage | PASS/FAIL | |
| Commit hygiene | PASS/FAIL | |
| /review-code | PASS/FAIL | Grade: A/B/C/D/F |

## PR Description (for /create-pr)

### What
### Why
### Notes

## Next Steps

1. Review the diff
2. If approved: `claude-sandbox ship`

## Blocking Issues (if BLOCKED)

### Issue 1: <description>
**Gate:** <which gate>
**Attempts:** <n>
**Error:** <output>
**What I tried:** <list>
**Likely cause:** <diagnosis>
```

## Container Image

### Base Image (apko)

Fat base image with full dev toolchain:

**Runtimes:**
- Node.js 22 + npm
- Go (latest)
- Python 3.12 + pip

**Cloud/infra tools:**
- gh CLI
- chainctl
- kubectl
- terraform

**Dev tools:**
- git, make, shellcheck
- golangci-lint
- curl, wget, jq, yq
- ripgrep, fd, fzf, tree
- coreutils, findutils, grep, sed, awk

**Build:**
```bash
apko build claude-code.apko.yaml claude-sandbox:base claude-sandbox-base.tar
docker load < claude-sandbox-base.tar

# Pre-bake Claude Code
docker build -t claude-sandbox:latest - <<'EOF'
FROM claude-sandbox:base
RUN npm install -g @anthropic-ai/claude-code
EOF
```

### Volume Mounts

**Read-only (host config):**
- `~/.claude/settings.json`
- `~/.claude/hooks/`
- `~/.claude/commands/`
- `~/.claude/skills/`
- `~/.gitconfig`
- `~/.ssh/`
- `~/.config/gh/`
- `~/.config/chainctl/`

**Read-write:**
- Worktree directory → `/workspace`
- Named volume `claude-history` → `/home/claude/.claude/history`

**Not mounted (isolated):**
- `~/.claude/projects/`
- `~/.kube/`
- Docker socket

## Implementation

### Language

Go (single binary, no runtime dependencies)

### Dependencies

- `cobra` for CLI structure
- `viper` for config parsing
- Standard library for docker/git operations

### Agents

- `golang-pro`: Implementation
- `test-automator`: Testing

## Open Source

All components use publicly available packages:
- apko (open source)
- Wolfi packages (public registry)
- Claude Code (public npm)
- chainctl (publicly available)

No private registry access required.

## Future Considerations

Not in scope for initial implementation:
- Multiple concurrent sandbox sessions
- Remote execution (cloud VMs)
- Team/shared sandbox environments
- Integration with CI/CD pipelines
