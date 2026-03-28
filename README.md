# claude-sandbox

[![CI](https://github.com/dakaneye/claude-sandbox/actions/workflows/ci.yml/badge.svg)](https://github.com/dakaneye/claude-sandbox/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/dakaneye/claude-sandbox)](https://github.com/dakaneye/claude-sandbox/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Run Claude Code autonomously in sandboxed Docker containers with git worktree isolation.

## What it does

You describe what you want built. Claude implements it in an isolated container, iterates on quality gates (build, lint, test, code review), and produces a branch ready for PR.

```
spec → execute → status → ship → clean
```

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Claude CLI](https://docs.anthropic.com/en/docs/claude-code) (`npm install -g @anthropic-ai/claude-code`)
- `ANTHROPIC_API_KEY` environment variable

## Install

**From release binary** (recommended):

```bash
# Download from GitHub Releases
# https://github.com/dakaneye/claude-sandbox/releases/latest

# macOS arm64
curl -L https://github.com/dakaneye/claude-sandbox/releases/latest/download/claude-sandbox_$(curl -s https://api.github.com/repos/dakaneye/claude-sandbox/releases/latest | grep tag_name | cut -d'"' -f4 | sed 's/v//')_darwin_arm64.tar.gz | tar xz
mv claude-sandbox /usr/local/bin/
```

**From source:**

```bash
go install github.com/dakaneye/claude-sandbox/cmd/claude-sandbox@latest
```

**Container image** (auto-pulled on first `execute`):

```bash
# Pulled automatically, or manually:
docker pull ghcr.io/dakaneye/claude-sandbox:latest

# Or build locally (requires apko):
claude-sandbox build
```

## Quick Start

```bash
export ANTHROPIC_API_KEY=sk-ant-...

# 1. Create a session — launches interactive Claude for planning
claude-sandbox spec --name my-feature

# 2. Execute — Claude implements the spec in a container
claude-sandbox execute

# 3. Check progress
claude-sandbox status

# 4. Review and create PR
claude-sandbox ship

# 5. Clean up
claude-sandbox clean
```

## Commands

| Command | Description |
|---------|-------------|
| `spec` | Create worktree + session, launch Claude for planning |
| `execute` | Run Claude in container to implement the spec |
| `status` | Show session status with progress analysis |
| `list` | List all sessions (alias: `ls`) |
| `ship` | Review COMPLETION.md and create PR |
| `stop` | Stop a running container |
| `clean` | Remove sessions and worktrees |
| `build` | Build the sandbox container image |

### spec

```bash
claude-sandbox spec --name my-feature              # auto-generated branch
claude-sandbox spec --name my-feature --branch dakaneye/my-feature  # custom branch
```

Creates a git worktree and launches Claude interactively. Use `/brainstorming` and `/writing-plans` skills to create a `PLAN.md`. Session is marked "ready" when `PLAN.md` exists.

### execute

```bash
claude-sandbox execute                    # uses active session
claude-sandbox execute --session my-feature  # specific session
```

Runs Claude in a Docker container with the worktree mounted. Claude reads `PLAN.md` and iterates until all quality gates pass. Progress is logged to `~/.claude/sandbox-sessions/<id>.log`.

### status

```bash
claude-sandbox status
```

Shows session info, timestamps, and — for running sessions — uses Claude Haiku to analyze the log and estimate progress (phase, current activity, % to completion).

### list

```bash
claude-sandbox list   # or: claude-sandbox ls
```

```
ID                     NAME           BRANCH                             STATUS     AGE
*2026-03-27-be10a5     my-feature     dakaneye/my-feature                running    12m
 2026-03-27-f8258f     token-work     sandbox/2026-03-27-7d9a8f          success    3h
```

### ship

```bash
claude-sandbox ship
claude-sandbox ship --skip-review    # skip COMPLETION.md review prompt
claude-sandbox ship --keep-worktree  # don't clean up after shipping
```

Validates COMPLETION.md shows success, opens it for review, then launches Claude with `/create-pr` to create the pull request.

## How it works

1. **spec** creates a git worktree (isolated branch) and launches Claude interactively to plan the work into `PLAN.md`
2. **execute** spins up a Docker container with the worktree mounted, runs Claude with `--dangerously-skip-permissions` inside the container, and streams output to a log file
3. Claude reads the plan, implements it, and iterates on quality gates (build, lint, test, `/review-code` grade A) until done
4. Claude writes `COMPLETION.md` with the final status
5. **ship** validates completion and creates a PR via Claude on the host

The container has controlled mounts (worktree read-write, git config read-only, Claude settings pre-baked) and passes `ANTHROPIC_API_KEY` via environment inheritance.

## GitHub Integration

For private repo access inside the container, set `GITHUB_TOKEN`:

```bash
export GITHUB_TOKEN=$(gh auth token)   # or use octo-sts for scoped tokens
claude-sandbox execute
```

The token is passed to the container and configured as a git credential helper scoped to `github.com` only. See `examples/octo-sts-policy.yaml` for an Octo STS trust policy example.

## Quality Gates

Claude is prompted to pass these gates before writing `COMPLETION.md`:

1. `make build` or `go build`
2. `golangci-lint run`
3. `go test ./...`
4. `/review-code` grade A

These are advisory — Claude is instructed to follow them but enforcement is prompt-based. Always review changes before shipping.

## Image Verification

The container image is signed with [Sigstore](https://sigstore.dev) keyless signing and includes an SBOM and SLSA provenance attestation.

```bash
# Verify image signature
cosign verify ghcr.io/dakaneye/claude-sandbox:latest \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'

# Download SBOM
cosign download sbom ghcr.io/dakaneye/claude-sandbox:latest | jq .

# Verify SLSA provenance
cosign verify-attestation ghcr.io/dakaneye/claude-sandbox:latest \
  --type slsaprovenance \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp 'github.com/dakaneye/claude-sandbox'
```

## License

MIT
