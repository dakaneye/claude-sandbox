# claude-sandbox

Sandboxed execution environment for autonomous Claude Code implementation.

## Overview

`claude-sandbox` enables spec-driven development where Claude implements features autonomously in an isolated container with quality gates, while blocking external side effects until human review.

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

## Quality Gates

Claude cannot claim completion until all gates pass:

1. **Build** - Project builds successfully
2. **Lint** - No linting errors
3. **Test** - All tests pass
4. **Security** - No new vulnerabilities
5. **Spec coverage** - All spec items addressed
6. **Commit hygiene** - Atomic commits with conventional messages
7. **/review-code** - Grade A from code review

## External Action Blocking

The sandbox blocks actions that would affect external systems:

- `git push` - blocked
- `gh pr create` - blocked
- `gh issue comment` - blocked
- `curl -X POST` - blocked
- Linear MCP writes - blocked

Read operations are allowed (gh pr view, curl GET, etc.).

## Configuration

Create `.claude-sandbox.yaml` in your project to override gate commands:

```yaml
gates:
  build: "make build"
  lint: "make lint"
  test: "make test-all"
  security: "make security-scan"
retries: 5
timeout: "4h"
```

## License

MIT
