# claude-sandbox

Sandboxed execution environment for autonomous Claude Code implementation.

## Overview

`claude-sandbox` enables spec-driven development where Claude implements features autonomously in an isolated container. The tool provides:

- **Git worktree isolation** - Each sandbox runs in its own worktree with a dedicated branch
- **Container execution** - Claude runs inside a Docker container with controlled mounts
- **Advisory quality gates** - Claude is prompted to follow quality gates (build, lint, test, security, etc.)
- **Advisory action blocking** - PreToolUse hooks warn about external side effects

> **Note**: Quality gates and action blocking are advisory. Claude is instructed to follow them but enforcement is prompt-based, not hard-coded. Human review before `ship` remains essential.

## Installation

```bash
# Build from source
make build
make install

# Build container image (requires apko)
cd container && ./build.sh --load
```

## Quick Start

```bash
# 1. Initialize a worktree for isolated work
claude-sandbox init ~/dev/my-project

# 2. Create your spec in the worktree
cd ~/dev/my-project-sandbox-abc123
# ... write spec to ./docs/specs/feature/

# 3. Run Claude autonomously
claude-sandbox run --spec ./docs/specs/feature/

# 4. Review and ship when notified
cat COMPLETION.md
git diff main...HEAD
claude-sandbox ship
```

## Commands

| Command | Description |
|---------|-------------|
| `init <path>` | Create git worktree for sandboxed work |
| `run --spec <path>` | Launch Claude in container to implement spec |
| `ship` | Create PR after reviewing completed work |
| `status` | Show current session status |
| `logs [-f]` | View session logs |
| `stop` | Stop running session |
| `clean` | Remove stale worktrees |

## Advisory Quality Gates

Claude is prompted to follow these quality gates before claiming completion:

1. **Build** - Project builds successfully
2. **Lint** - No linting errors
3. **Test** - All tests pass
4. **Security** - No new vulnerabilities
5. **Spec coverage** - All spec items addressed
6. **Commit hygiene** - Atomic commits with conventional messages
7. **/review-code** - Grade A from code review

These are advisory instructions to Claude, not enforced checks.

## Advisory Action Blocking

The container includes PreToolUse hooks that advise Claude against external side effects:

- `git push` - advised against
- `gh pr create` - advised against
- `gh issue comment` - advised against
- `curl -X POST` - advised against
- Linear MCP writes - advised against

Read operations are allowed (gh pr view, curl GET, etc.).

> **Important**: Hooks are advisory. Claude can choose to ignore them. Always review changes before running `ship`.

## License

MIT
