# Scoped GitHub Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GITHUB_TOKEN environment variable pass-through with automatic gh/git credential configuration in the container.

**Architecture:** Pass GITHUB_TOKEN via Docker's `-e` flag (like ANTHROPIC_API_KEY). Prepend a bash setup script to `buildClaudeCommand()` that configures `gh auth` and `git credential.helper` when the token is present.

**Tech Stack:** Go, Docker, bash

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/container/container.go` | Modify | Add GITHUB_TOKEN env var, prepend credential setup to command |
| `internal/container/container_test.go` | Modify | Add tests for GITHUB_TOKEN pass-through and command setup |
| `examples/octo-sts-policy.yaml` | Create | Example Octo STS policy file with setup instructions |
| `README.md` | Modify | Add GitHub Integration section |

---

### Task 1: Add GITHUB_TOKEN to BuildRunArgs

**Files:**
- Modify: `internal/container/container.go:59-62`
- Test: `internal/container/container_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/container/container_test.go` after the existing `TestBuildRunArgs` function:

```go
func TestBuildRunArgs_GitHubToken(t *testing.T) {
	opts := RunOptions{
		Image:        "claude-sandbox:latest",
		WorktreePath: "/tmp/worktree",
		HomeDir:      "/Users/test",
		SpecPath:     "/tmp/worktree/spec.md",
	}

	args := BuildRunArgs(opts)

	// Should pass GITHUB_TOKEN via environment inheritance
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) && args[i+1] == "GITHUB_TOKEN" {
			return // Found it
		}
	}
	t.Error("missing GITHUB_TOKEN environment variable passthrough")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/container/... -run TestBuildRunArgs_GitHubToken -v`

Expected: FAIL with "missing GITHUB_TOKEN environment variable passthrough"

- [ ] **Step 3: Add GITHUB_TOKEN to BuildRunArgs**

In `internal/container/container.go`, find line 61-62:

```go
	args = append(args, "-e", "ANTHROPIC_API_KEY")
	args = append(args, "-e", "HOME=/home/claude")
```

Change to:

```go
	args = append(args, "-e", "ANTHROPIC_API_KEY")
	args = append(args, "-e", "GITHUB_TOKEN")
	args = append(args, "-e", "HOME=/home/claude")
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/container/... -run TestBuildRunArgs_GitHubToken -v`

Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./internal/container/... -v`

Expected: All tests pass

- [ ] **Step 6: Commit**

```bash
git add internal/container/container.go internal/container/container_test.go
git commit -m "feat(container): add GITHUB_TOKEN environment variable pass-through"
```

---

### Task 2: Add GitHub Credential Setup to buildClaudeCommand

**Files:**
- Modify: `internal/container/container.go:118-139`
- Test: `internal/container/container_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/container/container_test.go`:

```go
func TestBuildClaudeCommand_GitHubSetup(t *testing.T) {
	cmd := buildClaudeCommand("/workspace/spec.md")

	// Should contain GitHub credential setup
	if !strings.Contains(cmd, "GITHUB_TOKEN") {
		t.Error("command should contain GITHUB_TOKEN check")
	}
	if !strings.Contains(cmd, "gh auth login") {
		t.Error("command should contain gh auth login")
	}
	if !strings.Contains(cmd, "credential.helper") {
		t.Error("command should contain git credential.helper setup")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/container/... -run TestBuildClaudeCommand_GitHubSetup -v`

Expected: FAIL with "command should contain GITHUB_TOKEN check"

- [ ] **Step 3: Update buildClaudeCommand with GitHub setup**

In `internal/container/container.go`, replace the `buildClaudeCommand` function (lines 118-139):

```go
func buildClaudeCommand(specPath string) string {
	// Shell-escape the spec path to prevent injection
	escaped := shellEscape(specPath)

	// GitHub credential setup (runs only if GITHUB_TOKEN is set)
	githubSetup := `
if [ -n "$GITHUB_TOKEN" ]; then
    echo "$GITHUB_TOKEN" | gh auth login --with-token 2>/dev/null
    git config --global credential.helper '!f() { echo "password=$GITHUB_TOKEN"; }; f'
fi
`

	// Holistic prompt: assess current state, do only what's needed to reach grade A
	claudeCmd := fmt.Sprintf(`claude --dangerously-skip-permissions "Your goal: get this project to pass all quality gates with /review-code grade A.

Spec: %s

First, assess the current state:
- Check git status, existing code, COMPLETION.md, any prior work
- Identify what's already done vs what's blocking grade A

Then do only what's needed:
- If not implemented, implement the spec
- If implemented but failing gates, fix the failures
- If passing gates but review grade < A, fix only the review feedback
- If grade A, verify gates still pass

Quality gates: build, lint, test, /review-code grade A.
Keep iterating on review feedback until grade A (do not stop at B or lower).
Update COMPLETION.md with final status."`, escaped)

	return githubSetup + claudeCmd
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/container/... -run TestBuildClaudeCommand_GitHubSetup -v`

Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./internal/container/... -v`

Expected: All tests pass

- [ ] **Step 6: Run build and lint**

Run: `go build ./... && golangci-lint run ./...`

Expected: Build succeeds, no lint errors

- [ ] **Step 7: Commit**

```bash
git add internal/container/container.go internal/container/container_test.go
git commit -m "feat(container): add GitHub credential setup to container command"
```

---

### Task 3: Create Octo STS Example Policy

**Files:**
- Create: `examples/octo-sts-policy.yaml`

- [ ] **Step 1: Create examples directory**

Run: `mkdir -p examples`

- [ ] **Step 2: Create the example policy file**

Create `examples/octo-sts-policy.yaml`:

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

- [ ] **Step 3: Verify file exists and is valid YAML**

Run: `cat examples/octo-sts-policy.yaml && yq '.' examples/octo-sts-policy.yaml`

Expected: File contents displayed, yq parses without error

- [ ] **Step 4: Commit**

```bash
git add examples/octo-sts-policy.yaml
git commit -m "docs: add Octo STS example policy for GitHub integration"
```

---

### Task 4: Update README with GitHub Integration Section

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add GitHub Integration section to README**

In `README.md`, add the following section before the "## License" section (around line 103):

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

When `GITHUB_TOKEN` is set, the container automatically configures `gh` CLI and `git` credentials. Host `~/.config/gh` is still mounted as fallback for repos not covered by the token.

```

- [ ] **Step 2: Verify README renders correctly**

Run: `head -120 README.md | tail -20`

Expected: GitHub Integration section visible

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: add GitHub Integration section to README"
```

---

### Task 5: Final Verification

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`

Expected: All tests pass

- [ ] **Step 2: Run build**

Run: `go build ./...`

Expected: Build succeeds

- [ ] **Step 3: Run lint**

Run: `golangci-lint run ./...`

Expected: No lint errors

- [ ] **Step 4: Verify git status**

Run: `git log --oneline -5`

Expected: 4 commits for this feature:
1. feat(container): add GITHUB_TOKEN environment variable pass-through
2. feat(container): add GitHub credential setup to container command
3. docs: add Octo STS example policy for GitHub integration
4. docs: add GitHub Integration section to README

- [ ] **Step 5: Manual integration test (optional)**

```bash
# Set a test token (can be invalid for this test)
export GITHUB_TOKEN="test-token"

# Build and install
make install

# Check that docker args would include GITHUB_TOKEN
# (Can't fully test without running container, but code paths are unit tested)
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Add GITHUB_TOKEN to BuildRunArgs | container.go, container_test.go |
| 2 | Add GitHub credential setup to buildClaudeCommand | container.go, container_test.go |
| 3 | Create Octo STS example policy | examples/octo-sts-policy.yaml |
| 4 | Update README | README.md |
| 5 | Final verification | - |
