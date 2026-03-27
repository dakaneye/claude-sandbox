# Scoped GitHub Access via Octo STS

**Issue:** [#6](https://github.com/dakaneye/claude-sandbox/issues/6)
**Date:** 2026-03-27
**Status:** Approved

## Problem

Users who want Claude to interact with GitHub from within the sandbox must pass their personal GitHub credentials into the container. This creates security risks:

- Full user credentials grant access to all repos the user can access
- Credentials have write permissions (push, merge, delete)
- Long-lived tokens that persist beyond the session
- User's identity is attached to any actions taken

## Solution

Support scoped, short-lived, read-only GitHub tokens via environment variable pass-through. Users obtain tokens externally (e.g., via Octo STS) and pass them to the container.

### Token Characteristics

- **Read-only**: `contents:read`, `pull_requests:read`, `issues:read`, `actions:read`
- **Repo-scoped**: Only repos explicitly listed in the Octo STS policy
- **Short-lived**: 1-hour expiry (Octo STS default)
- **Service identity**: Not tied to user's personal GitHub account

## Design

### Data Flow

```
User sets GITHUB_TOKEN env var
         │
         ▼
claude-sandbox execute
         │
         ▼
container.BuildRunArgs() adds "-e GITHUB_TOKEN"
         │
         ▼
Container starts with GITHUB_TOKEN in environment
         │
         ▼
Startup script detects GITHUB_TOKEN is set
         │
         ├─► gh auth login --with-token <<< "$GITHUB_TOKEN"
         │
         └─► git config --global credential.helper \
               '!f() { echo "password=$GITHUB_TOKEN"; }; f'
         │
         ▼
Claude runs with gh/git configured
```

### Fallback Behavior

- If `GITHUB_TOKEN` not set: no startup script runs, host `~/.config/gh` mount used as-is
- If `GITHUB_TOKEN` set: token takes precedence, but host mount still available for repos not covered by token

## Implementation

### Code Changes

**internal/container/container.go**

1. Add `GITHUB_TOKEN` to environment pass-through in `BuildRunArgs()`:

```go
// After ANTHROPIC_API_KEY line
args = append(args, "-e", "GITHUB_TOKEN")
```

2. Modify `buildClaudeCommand()` to prepend GitHub credential setup:

```go
func buildClaudeCommand(specPath string) string {
    escaped := shellEscape(specPath)

    // GitHub credential setup (runs only if GITHUB_TOKEN is set)
    githubSetup := `
if [ -n "$GITHUB_TOKEN" ]; then
    echo "$GITHUB_TOKEN" | gh auth login --with-token 2>/dev/null
    git config --global credential.helper '!f() { echo "password=$GITHUB_TOKEN"; }; f'
fi
`
    // Existing Claude command (unchanged)
    claudeCmd := fmt.Sprintf(`claude --dangerously-skip-permissions "Your goal: ..."`, escaped)

    return githubSetup + claudeCmd
}
```

Note: The `claudeCmd` content remains unchanged from the existing implementation. Only the `githubSetup` prefix is added.

### No Changes Required

- `mounts.go`: Host `~/.config/gh` still mounted as fallback
- CLI commands: No new flags (auto-enable when token present)
- Container image: `gh` CLI already installed
- State management: No persistence needed

### New Files

**examples/octo-sts-policy.yaml**

```yaml
# Octo STS trust policy for claude-sandbox GitHub access
#
# Location: .github/chainguard/{name}.sts.yaml in your repo
# (The "chainguard" directory name is fixed, regardless of OIDC issuer)
#
# See: https://github.com/octo-sts/app
#
# Usage:
# 1. Install Octo STS GitHub App on your org/repos
# 2. Copy this to .github/chainguard/claude.sts.yaml in target repo
# 3. Get token: octo-sts get --scope org/repo --identity claude
# 4. Export: export GITHUB_TOKEN=$(octo-sts get ...)
# 5. Run: claude-sandbox execute

issuer: https://accounts.google.com
# issuer: https://token.actions.githubusercontent.com
subject: user@example.com

permissions:
  contents: read
  pull_requests: read
  issues: read
  actions: read
```

**README.md addition**

```markdown
## GitHub Integration

To give Claude scoped, read-only GitHub access without passing your personal credentials:

1. Set up [Octo STS](https://github.com/octo-sts/app) for your repositories
2. See `examples/octo-sts-policy.yaml` for a sample policy
3. Export the token before running:
   ```bash
   export GITHUB_TOKEN=$(octo-sts get --scope org/repo ...)
   claude-sandbox execute
   ```
```

## Testing

### Unit Tests

- Verify `BuildRunArgs` includes `-e GITHUB_TOKEN` in returned args

### Integration Tests (Manual)

```bash
# Test 1: Token passed and gh configured
export GITHUB_TOKEN=$(octo-sts get --scope org/repo --identity claude)
claude-sandbox execute --session test
# Inside container: gh auth status → should show "Logged in"

# Test 2: No token, fallback to host config
unset GITHUB_TOKEN
claude-sandbox execute --session test
# Inside container: gh auth status → should use host ~/.config/gh

# Test 3: Invalid token, graceful degradation
export GITHUB_TOKEN="invalid"
claude-sandbox execute --session test
# gh auth login fails silently, Claude continues without GitHub access
```

## Scope

Small, focused change:
- ~20 lines of Go code
- 1 new example file
- README section addition
- Unit test additions

## Alternatives Considered

1. **Token file mount**: User creates token file, container mounts it. More complex, no clear benefit over env var.

2. **Container entrypoint script**: Separate script file for credential setup. Adds complexity to container build.

3. **Explicit --github flag**: Require flag to enable GitHub integration. Adds friction with no benefit since token presence is sufficient signal.

Selected approach (env var + inline setup script) is simplest for users and requires minimal code changes.
