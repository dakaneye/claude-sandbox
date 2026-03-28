# Completion Status Parsing Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix false-negative status detection in execute and ship commands by replacing brittle exact substring matching with resilient two-pass parsing of COMPLETION.md.

**Architecture:** New `completion.go` with a shared `parseCompletionStatus` function (strict line-match + fuzzy keyword fallback). Both `execute.go` and `ship.go` call it instead of inline `strings.Contains`. Prompt updated to guide Claude toward a parseable format.

**Tech Stack:** Go standard library only.

**Spec:** `docs/specs/2026-03-27-completion-status-parsing-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/cli/completion.go` | `parseCompletionStatus(content string) state.Status` |
| `internal/cli/completion_test.go` | Table-driven tests for strict, fuzzy, edge cases |
| `internal/cli/execute.go:176-183` | Replace inline matching with `parseCompletionStatus` |
| `internal/cli/ship.go:69-71` | Replace inline matching with `parseCompletionStatus` |
| `internal/container/container.go:143-144` | Update COMPLETION.md instruction in prompt |

---

### Task 1: Create parseCompletionStatus with tests

**Files:**
- Create: `internal/cli/completion.go`
- Create: `internal/cli/completion_test.go`

- [ ] **Step 1: Write the test file**

```go
// internal/cli/completion_test.go
package cli

import (
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestParseCompletionStatus(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected state.Status
	}{
		{
			name:     "strict SUCCESS",
			content:  "STATUS: SUCCESS\n\nAll gates passed.",
			expected: state.StatusSuccess,
		},
		{
			name:     "strict BLOCKED",
			content:  "STATUS: BLOCKED\n\nLint failures.",
			expected: state.StatusBlocked,
		},
		{
			name:     "strict FAILED",
			content:  "STATUS: FAILED\n\nBuild broken.",
			expected: state.StatusFailed,
		},
		{
			name:     "strict lowercase",
			content:  "status: success\n\nDone.",
			expected: state.StatusSuccess,
		},
		{
			name:     "strict with whitespace",
			content:  "  Status:  SUCCESS  \n\nDone.",
			expected: state.StatusSuccess,
		},
		{
			name:     "strict complete maps to success",
			content:  "Status: Complete\n\nDone.",
			expected: state.StatusSuccess,
		},
		{
			name:     "fuzzy markdown bold status",
			content:  "# Completion\n\n**Status**: Complete — Grade A\n\nDetails...",
			expected: state.StatusSuccess,
		},
		{
			name:     "fuzzy success in body",
			content:  "# Result\n\nAll quality gates passed successfully.\n\nGrade A.",
			expected: state.StatusSuccess,
		},
		{
			name:     "fuzzy blocked",
			content:  "# Result\n\nCould not proceed, blocked by lint failures.",
			expected: state.StatusBlocked,
		},
		{
			name:     "blocked beats success",
			content:  "Blocked from reaching success.",
			expected: state.StatusBlocked,
		},
		{
			name:     "old format still works",
			content:  "## Status: SUCCESS\n\nImplemented feature.",
			expected: state.StatusSuccess,
		},
		{
			name:     "no match defaults to failed",
			content:  "# Some random markdown\n\nNo status here.",
			expected: state.StatusFailed,
		},
		{
			name:     "empty content",
			content:  "",
			expected: state.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCompletionStatus(tt.content)
			if got != tt.expected {
				t.Errorf("parseCompletionStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestParseCompletionStatus -v`
Expected: FAIL — `parseCompletionStatus` not defined

- [ ] **Step 3: Write the implementation**

```go
// internal/cli/completion.go
package cli

import (
	"strings"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

// parseCompletionStatus determines session status from COMPLETION.md content.
// Strict pass: looks for a line starting with "status:" (case-insensitive).
// Fuzzy fallback: scans for keywords. Priority: blocked > success > failed.
func parseCompletionStatus(content string) state.Status {
	// Strict pass: find a line starting with "status:"
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Strip markdown bold markers
		trimmed = strings.ReplaceAll(trimmed, "**", "")
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, "status:") {
			continue
		}
		value := strings.TrimSpace(trimmed[len("status:"):])
		value = strings.ToLower(value)
		// Strip trailing markdown (e.g., "Complete — Grade A" → "complete")
		value = strings.SplitN(value, "—", 2)[0]
		value = strings.SplitN(value, "-", 2)[0]
		value = strings.TrimSpace(value)

		switch {
		case value == "success" || value == "complete":
			return state.StatusSuccess
		case value == "blocked":
			return state.StatusBlocked
		case value == "failed":
			return state.StatusFailed
		}
	}

	// Fuzzy fallback: keyword scan with priority
	lower := strings.ToLower(content)
	hasBlocked := strings.Contains(lower, "blocked")
	hasSuccess := strings.Contains(lower, "success") || strings.Contains(lower, "complete")
	// hasFailed not checked — it's the default

	if hasBlocked {
		return state.StatusBlocked
	}
	if hasSuccess {
		return state.StatusSuccess
	}
	return state.StatusFailed
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestParseCompletionStatus -v`
Expected: All PASS

- [ ] **Step 5: Run build**

Run: `go build ./...`
Expected: Clean

- [ ] **Step 6: Commit**

```bash
git add internal/cli/completion.go internal/cli/completion_test.go
git commit -m "feat(status): add parseCompletionStatus with strict and fuzzy matching"
```

---

### Task 2: Update execute.go to use parseCompletionStatus

**Files:**
- Modify: `internal/cli/execute.go:176-183`

- [ ] **Step 1: Replace the inline matching block**

In `internal/cli/execute.go`, replace lines 175-189:

```go
			// Read COMPLETION.md to determine final status
			if content, err := os.ReadFile(completionPath); err == nil {
				if strings.Contains(string(content), "Status: SUCCESS") {
					sess.Status = state.StatusSuccess
				} else if strings.Contains(string(content), "Status: BLOCKED") {
					sess.Status = state.StatusBlocked
				} else {
					sess.Status = state.StatusFailed
				}
			} else if runErr != nil {
				sess.Status = state.StatusFailed
				sess.Error = runErr.Error()
			} else {
				sess.Status = state.StatusFailed
			}
```

With:

```go
			// Read COMPLETION.md to determine final status
			if content, err := os.ReadFile(completionPath); err == nil {
				sess.Status = parseCompletionStatus(string(content))
			} else if runErr != nil {
				sess.Status = state.StatusFailed
				sess.Error = runErr.Error()
			} else {
				sess.Status = state.StatusFailed
			}
```

- [ ] **Step 2: Remove unused `strings` import if no other references**

Check if `strings` is still used in execute.go. If only used for the removed `strings.Contains` calls, remove it from the import block.

Run: `grep -n 'strings\.' internal/cli/execute.go`

If the only hit was the removed lines, remove `"strings"` from imports.

- [ ] **Step 3: Run tests and build**

Run: `go test ./internal/cli/ -v -count=1 && go build ./...`
Expected: All PASS, clean build

- [ ] **Step 4: Commit**

```bash
git add internal/cli/execute.go
git commit -m "fix(execute): use parseCompletionStatus for resilient status detection"
```

---

### Task 3: Update ship.go to use parseCompletionStatus

**Files:**
- Modify: `internal/cli/ship.go:69-71`

- [ ] **Step 1: Replace the inline matching**

In `internal/cli/ship.go`, replace lines 69-71:

```go
	if !strings.Contains(string(content), "Status: SUCCESS") {
		return fmt.Errorf("COMPLETION.md does not show SUCCESS status. Cannot ship blocked or failed work")
	}
```

With:

```go
	if parseCompletionStatus(string(content)) != state.StatusSuccess {
		return fmt.Errorf("COMPLETION.md does not show SUCCESS status. Cannot ship blocked or failed work")
	}
```

- [ ] **Step 2: Remove unused `strings` import if no other references**

Run: `grep -n 'strings\.' internal/cli/ship.go`

If `strings` is no longer used, remove it from the import block.

- [ ] **Step 3: Run tests and build**

Run: `go test ./internal/cli/ -v -count=1 && go build ./...`
Expected: All PASS, clean build

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ship.go
git commit -m "fix(ship): use parseCompletionStatus for resilient status detection"
```

---

### Task 4: Update buildClaudeCommand prompt

**Files:**
- Modify: `internal/container/container.go:143-144`

- [ ] **Step 1: Update the COMPLETION.md instruction in the prompt**

In `internal/container/container.go`, in the `buildClaudeCommand` function, replace:

```go
Update COMPLETION.md with final status."`, escaped)
```

With:

```go
When done, write COMPLETION.md. The FIRST LINE must be exactly one of:
STATUS: SUCCESS
STATUS: BLOCKED
STATUS: FAILED
Follow with details about what was done and quality gate results."`, escaped)
```

- [ ] **Step 2: Update the container test assertion**

In `internal/container/container_test.go`, in `TestBuildClaudeCommand`, add an assertion for the new prompt format:

```go
	if !strings.Contains(cmd, "STATUS: SUCCESS") {
		t.Error("command should contain STATUS: SUCCESS instruction")
	}
```

- [ ] **Step 3: Run tests and build**

Run: `go test ./internal/container/ -v -count=1 && go build ./...`
Expected: All PASS, clean build

- [ ] **Step 4: Commit**

```bash
git add internal/container/container.go internal/container/container_test.go
git commit -m "fix(container): update prompt to request structured COMPLETION.md status line"
```

---

### Task 5: Final quality gates

**Files:** None (verification only)

- [ ] **Step 1: Full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 2: Lint**

Run: `golangci-lint run ./...`
Expected: 0 issues

- [ ] **Step 3: Build**

Run: `make build`
Expected: PASS

- [ ] **Step 4: Tidy**

Run: `go mod tidy && git diff --exit-code go.mod go.sum`
Expected: No changes

- [ ] **Step 5: Verify against the real COMPLETION.md that triggered this bug**

Run in a Go test or manually:
```go
content := `**Status**: Complete — Grade A`
fmt.Println(parseCompletionStatus(content)) // should print "success"
```
