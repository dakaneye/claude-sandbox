# Status Log Analysis Fix — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the status command crash by parsing stream-json logs into a structured summary before sending to haiku for progress estimation.

**Architecture:** New `logsummary.go` handles JSON parsing and summary generation. Updated `analyze.go` accepts a formatted summary string instead of raw log content. `status.go` orchestrates: parse → format → analyze, with graceful degradation at each step.

**Tech Stack:** Go standard library (`encoding/json`, `bufio`, `os`), existing `claude` CLI for haiku analysis.

**Spec:** `docs/specs/2026-03-27-status-log-analysis-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/cli/logsummary.go` | `LogSummary` struct, `parseLogEvents()`, `formatSummary()`, `formatFallback()` |
| `internal/cli/logsummary_test.go` | Unit tests for parsing, formatting, gate detection |
| `internal/cli/analyze.go` | Updated `analyzeLog()` accepting summary string, updated prompt, error reason returns |
| `internal/cli/analyze_test.go` | New tests for `claudeAvailable` and prompt construction |
| `internal/cli/status.go` | Updated `StatusRunning` case using new flow |
| `internal/cli/logutil.go` | Deleted (replaced by `logsummary.go`) |
| `internal/cli/logutil_test.go` | Deleted (replaced by `logsummary_test.go`) |
| `test/e2e/workflow_test.sh` | E2E test: spec → execute → status → clean |

---

### Task 1: Create LogSummary struct and parseLogEvents

**Files:**
- Create: `internal/cli/logsummary.go`
- Test: `internal/cli/logsummary_test.go`

- [ ] **Step 1: Write failing test for empty log file**

```go
// internal/cli/logsummary_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLogEvents_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	summary, err := parseLogEvents(path, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalTools != 0 {
		t.Errorf("TotalTools = %d, want 0", summary.TotalTools)
	}
	if len(summary.ToolCounts) != 0 {
		t.Errorf("ToolCounts = %v, want empty", summary.ToolCounts)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestParseLogEvents_Empty -v`
Expected: FAIL — `parseLogEvents` not defined

- [ ] **Step 3: Write LogSummary struct and minimal parseLogEvents**

```go
// internal/cli/logsummary.go
package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// LogSummary holds structured metrics extracted from a stream-json log.
type LogSummary struct {
	ElapsedTime  time.Duration
	TotalTools   int
	ToolCounts   map[string]int
	LastTool     string
	LastToolDesc string
	LastToolTime time.Time
	TaskProgress []string
	GateMentions map[string]bool
}

// logEvent is the minimal JSON structure we extract from each line.
// Fields are pointers/omitempty so missing fields decode to zero values.
type logEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	// For system messages
	Description string `json:"description"`
	// For assistant messages with tool_use content
	Message *logMessage `json:"message"`
	// Timestamp on user messages (tool results)
	Timestamp string `json:"timestamp"`
}

type logMessage struct {
	Content []logContent `json:"content"`
}

type logContent struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// toolInput extracts description and command from Bash tool inputs.
type toolInput struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

// maxTaskProgress is the number of recent task_progress descriptions to keep.
const maxTaskProgress = 3

// gatePatterns maps gate names to keywords that indicate the gate was attempted.
var gatePatterns = map[string][]string{
	"build":       {"make build", "go build"},
	"lint":        {"golangci-lint", "lint"},
	"test":        {"go test"},
	"review-code": {"review-code", "/review"},
}

// parseLogEvents reads a stream-json log file and extracts structured metrics.
// Malformed lines are skipped. Returns a zero-value summary (not an error) for
// empty or unreadable files.
func parseLogEvents(path string, startedAt time.Time) (*LogSummary, error) {
	summary := &LogSummary{
		ElapsedTime:  time.Since(startedAt),
		ToolCounts:   make(map[string]int),
		GateMentions: make(map[string]bool),
	}

	file, err := os.Open(path)
	if err != nil {
		return summary, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		var event logEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // skip malformed lines
		}

		switch event.Type {
		case "assistant":
			processAssistantEvent(summary, &event)
		case "system":
			processSystemEvent(summary, &event)
		case "user":
			processUserEvent(summary, &event)
		}
	}

	return summary, nil
}

func processAssistantEvent(s *LogSummary, event *logEvent) {
	if event.Message == nil {
		return
	}
	for _, content := range event.Message.Content {
		if content.Type != "tool_use" {
			continue
		}
		s.TotalTools++
		s.ToolCounts[content.Name]++
		s.LastTool = content.Name

		// Extract description from tool input
		var input toolInput
		if err := json.Unmarshal(content.Input, &input); err == nil {
			if input.Description != "" {
				s.LastToolDesc = input.Description
			} else if input.Command != "" {
				// Truncate long commands
				cmd := input.Command
				if len(cmd) > 80 {
					cmd = cmd[:77] + "..."
				}
				s.LastToolDesc = cmd
			}
		}

		// Check for gate mentions in Bash commands
		if content.Name == "Bash" {
			checkGates(s, content.Input)
		}
	}
}

func processSystemEvent(s *LogSummary, event *logEvent) {
	if event.Subtype != "task_progress" || event.Description == "" {
		return
	}
	if len(s.TaskProgress) >= maxTaskProgress {
		s.TaskProgress = s.TaskProgress[1:]
	}
	s.TaskProgress = append(s.TaskProgress, event.Description)
}

func processUserEvent(s *LogSummary, event *logEvent) {
	if event.Timestamp == "" {
		return
	}
	t, err := time.Parse(time.RFC3339Nano, event.Timestamp)
	if err != nil {
		// Try alternate format
		t, err = time.Parse("2006-01-02T15:04:05.000Z", event.Timestamp)
		if err != nil {
			return
		}
	}
	s.LastToolTime = t
}

func checkGates(s *LogSummary, raw json.RawMessage) {
	var input toolInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return
	}
	combined := strings.ToLower(input.Command + " " + input.Description)
	for gate, patterns := range gatePatterns {
		for _, pattern := range patterns {
			if strings.Contains(combined, pattern) {
				s.GateMentions[gate] = true
				break
			}
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestParseLogEvents_Empty -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/logsummary.go internal/cli/logsummary_test.go
git commit -m "feat(status): add LogSummary struct and parseLogEvents for empty files"
```

---

### Task 2: Add tool counting and malformed JSON tests

**Files:**
- Modify: `internal/cli/logsummary_test.go`

- [ ] **Step 1: Write failing test for tool counting**

Add to `internal/cli/logsummary_test.go`:

```go
func TestParseLogEvents_ToolCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lines := []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"make build","description":"Run build"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/workspace/main.go"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./...","description":"Run tests"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/workspace/main.go"}}]}}`,
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	summary, err := parseLogEvents(path, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalTools != 4 {
		t.Errorf("TotalTools = %d, want 4", summary.TotalTools)
	}
	if summary.ToolCounts["Bash"] != 2 {
		t.Errorf("Bash count = %d, want 2", summary.ToolCounts["Bash"])
	}
	if summary.ToolCounts["Read"] != 1 {
		t.Errorf("Read count = %d, want 1", summary.ToolCounts["Read"])
	}
	if summary.ToolCounts["Edit"] != 1 {
		t.Errorf("Edit count = %d, want 1", summary.ToolCounts["Edit"])
	}
	if summary.LastTool != "Edit" {
		t.Errorf("LastTool = %q, want %q", summary.LastTool, "Edit")
	}
}

func TestParseLogEvents_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lines := []string{
		`not json at all`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"broken json`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/a"}}]}}`,
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	summary, err := parseLogEvents(path, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalTools != 2 {
		t.Errorf("TotalTools = %d, want 2 (should skip malformed lines)", summary.TotalTools)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run "TestParseLogEvents_(ToolCounts|MalformedJSON)" -v`
Expected: PASS (implementation already handles these cases)

- [ ] **Step 3: Commit**

```bash
git add internal/cli/logsummary_test.go
git commit -m "test(status): add tool counting and malformed JSON tests"
```

---

### Task 3: Add gate detection and task progress tests

**Files:**
- Modify: `internal/cli/logsummary_test.go`

- [ ] **Step 1: Write failing test for gate detection**

Add to `internal/cli/logsummary_test.go`:

```go
func TestParseLogEvents_GateDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lines := []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"make build 2>&1 && echo BUILD PASS","description":"Run build"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"golangci-lint run ./...","description":"Run lint"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./... 2>&1","description":"Run tests"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/workspace/main.go"}}]}}`,
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	summary, err := parseLogEvents(path, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, gate := range []string{"build", "lint", "test"} {
		if !summary.GateMentions[gate] {
			t.Errorf("gate %q not detected", gate)
		}
	}
	if summary.GateMentions["review-code"] {
		t.Error("review-code gate should not be detected")
	}
}

func TestParseLogEvents_TaskProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	lines := []string{
		`{"type":"system","subtype":"task_progress","description":"Reading main.go"}`,
		`{"type":"system","subtype":"task_progress","description":"Running build"}`,
		`{"type":"system","subtype":"task_progress","description":"Running tests"}`,
		`{"type":"system","subtype":"task_progress","description":"Fixing lint errors"}`,
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	summary, err := parseLogEvents(path, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(summary.TaskProgress) != 3 {
		t.Fatalf("TaskProgress length = %d, want 3 (last 3 only)", len(summary.TaskProgress))
	}
	if summary.TaskProgress[0] != "Running build" {
		t.Errorf("TaskProgress[0] = %q, want %q", summary.TaskProgress[0], "Running build")
	}
	if summary.TaskProgress[2] != "Fixing lint errors" {
		t.Errorf("TaskProgress[2] = %q, want %q", summary.TaskProgress[2], "Fixing lint errors")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run "TestParseLogEvents_(GateDetection|TaskProgress)" -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/cli/logsummary_test.go
git commit -m "test(status): add gate detection and task progress tests"
```

---

### Task 4: Add formatSummary and formatFallback

**Files:**
- Modify: `internal/cli/logsummary.go`
- Modify: `internal/cli/logsummary_test.go`

- [ ] **Step 1: Write failing test for formatSummary**

Add to `internal/cli/logsummary_test.go`:

```go
func TestFormatSummary(t *testing.T) {
	summary := &LogSummary{
		ElapsedTime: 12 * time.Minute,
		TotalTools:  47,
		ToolCounts: map[string]int{
			"Bash": 15,
			"Read": 20,
			"Edit": 8,
			"Task": 4,
		},
		LastTool:     "Bash",
		LastToolDesc: "go test ./...",
		LastToolTime: time.Now().Add(-15 * time.Second),
		TaskProgress: []string{
			"Running tests in internal/cli",
			"Fixing lint errors",
		},
		GateMentions: map[string]bool{
			"build": true,
			"lint":  true,
			"test":  true,
		},
	}

	result := formatSummary(summary)

	checks := []string{
		"12m0s elapsed",
		"47 tool calls",
		"Bash(15)",
		"Read(20)",
		"Edit(8)",
		"Last: Bash",
		"go test ./...",
		"Running tests in internal/cli",
		"Fixing lint errors",
		"build",
		"lint",
		"test",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("formatSummary missing %q in:\n%s", check, result)
		}
	}
}

func TestFormatFallback(t *testing.T) {
	summary := &LogSummary{
		ElapsedTime: 5 * time.Minute,
		TotalTools:  10,
		ToolCounts: map[string]int{
			"Bash": 5,
			"Read": 5,
		},
		LastTool:     "Bash",
		LastToolDesc: "make build",
		GateMentions: map[string]bool{
			"build": true,
		},
	}

	result := formatFallback(summary, "claude CLI timeout")

	if !strings.Contains(result, "Analysis unavailable: claude CLI timeout") {
		t.Errorf("missing warning in:\n%s", result)
	}
	if !strings.Contains(result, "10 tool calls") {
		t.Errorf("missing tool count in:\n%s", result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run "TestFormat(Summary|Fallback)" -v`
Expected: FAIL — `formatSummary` and `formatFallback` not defined

- [ ] **Step 3: Implement formatSummary and formatFallback**

Add to `internal/cli/logsummary.go`:

```go
// formatSummary produces a condensed text summary for haiku analysis.
func formatSummary(s *LogSummary) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Session: %s elapsed, %d tool calls\n", s.ElapsedTime.Round(time.Second), s.TotalTools)

	// Tool counts sorted by frequency
	if len(s.ToolCounts) > 0 {
		type toolCount struct {
			name  string
			count int
		}
		var sorted []toolCount
		for name, count := range s.ToolCounts {
			sorted = append(sorted, toolCount{name, count})
		}
		slices.SortFunc(sorted, func(a, b toolCount) int {
			return b.count - a.count
		})
		b.WriteString("Tools: ")
		for i, tc := range sorted {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s(%d)", tc.name, tc.count)
		}
		b.WriteString("\n")
	}

	// Last tool
	if s.LastTool != "" {
		b.WriteString("Last: ")
		b.WriteString(s.LastTool)
		if s.LastToolDesc != "" {
			fmt.Fprintf(&b, " %q", s.LastToolDesc)
		}
		if !s.LastToolTime.IsZero() {
			ago := time.Since(s.LastToolTime).Round(time.Second)
			fmt.Fprintf(&b, " (%s ago)", ago)
		}
		b.WriteString("\n")
	}

	// Task progress
	if len(s.TaskProgress) > 0 {
		b.WriteString("Recent progress:\n")
		for _, desc := range s.TaskProgress {
			fmt.Fprintf(&b, "- %s\n", desc)
		}
	}

	// Gates
	gates := []string{"build", "lint", "test", "review-code"}
	b.WriteString("Gates: ")
	for i, gate := range gates {
		if i > 0 {
			b.WriteString(" | ")
		}
		if s.GateMentions[gate] {
			fmt.Fprintf(&b, "%s seen", gate)
		} else {
			fmt.Fprintf(&b, "%s ?", gate)
		}
	}
	b.WriteString("\n")

	return b.String()
}

// formatFallback produces a raw metrics display with a warning when haiku analysis fails.
func formatFallback(s *LogSummary, reason string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "⚠ Analysis unavailable: %s\n\n", reason)

	fmt.Fprintf(&b, "Progress: %d tool calls over %s\n", s.TotalTools, s.ElapsedTime.Round(time.Second))

	if len(s.ToolCounts) > 0 {
		type toolCount struct {
			name  string
			count int
		}
		var sorted []toolCount
		for name, count := range s.ToolCounts {
			sorted = append(sorted, toolCount{name, count})
		}
		slices.SortFunc(sorted, func(a, b toolCount) int {
			return b.count - a.count
		})
		b.WriteString("Tools: ")
		for i, tc := range sorted {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s(%d)", tc.name, tc.count)
		}
		b.WriteString("\n")
	}

	if s.LastTool != "" {
		fmt.Fprintf(&b, "Last: %s", s.LastTool)
		if s.LastToolDesc != "" {
			fmt.Fprintf(&b, " %q", s.LastToolDesc)
		}
		b.WriteString("\n")
	}

	gates := []string{"build", "lint", "test", "review-code"}
	b.WriteString("Gates: ")
	for i, gate := range gates {
		if i > 0 {
			b.WriteString(" | ")
		}
		if s.GateMentions[gate] {
			b.WriteString(gate + " seen")
		} else {
			b.WriteString(gate + " ?")
		}
	}
	b.WriteString("\n")

	return b.String()
}
```

Add the `slices` import to the imports at the top of `logsummary.go`:

```go
import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run "TestFormat(Summary|Fallback)" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/logsummary.go internal/cli/logsummary_test.go
git commit -m "feat(status): add formatSummary and formatFallback for log analysis output"
```

---

### Task 5: Update analyzeLog to accept summary string and return error reason

**Files:**
- Modify: `internal/cli/analyze.go`

- [ ] **Step 1: Write failing test for updated analyzeLog**

Add to a new file `internal/cli/analyze_test.go`:

```go
// internal/cli/analyze_test.go
package cli

import "testing"

func TestAnalyzeLog_EmptyInput(t *testing.T) {
	analysis, reason := analyzeLog(t.Context(), "")
	if analysis != "" {
		t.Errorf("analysis = %q, want empty for empty input", analysis)
	}
	if reason != "no log data" {
		t.Errorf("reason = %q, want %q", reason, "no log data")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestAnalyzeLog_EmptyInput -v`
Expected: FAIL — `analyzeLog` returns one value, not two

- [ ] **Step 3: Update analyzeLog signature and implementation**

Replace `internal/cli/analyze.go` entirely:

```go
package cli

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const analysisPrompt = `Analyze this Claude Code session summary. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's likely doing now
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Summary:
`

// analyzeLog uses Claude haiku to analyze the summary content.
// Returns (analysis, "") on success, or ("", reason) on failure.
// Respects the provided context for cancellation with a 30s timeout.
func analyzeLog(ctx context.Context, summary string) (string, string) {
	if summary == "" {
		return "", "no log data"
	}

	prompt := analysisPrompt + summary

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", prompt)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", "claude CLI timeout"
		}
		return "", "claude CLI error"
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", "empty analysis response"
	}
	return result, ""
}

// claudeAvailable checks if the claude CLI is available.
func claudeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestAnalyzeLog_EmptyInput -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/analyze.go internal/cli/analyze_test.go
git commit -m "refactor(status): update analyzeLog to return error reason for degraded output"
```

---

### Task 6: Update status.go to use new parsing flow

**Files:**
- Modify: `internal/cli/status.go`

- [ ] **Step 1: Write failing test for running status with log file**

Add to `internal/cli/status_test.go` (add `"path/filepath"`, `"strings"`, and `"time"` to the import block, plus `"github.com/dakaneye/claude-sandbox/internal/state"` if not already present):

```go
func TestStatusCommand_RunningWithLog(t *testing.T) {
	repo := setupTestRepoForCLI(t)
	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Create log directory and file with sample stream-json content
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "test.log")
	logLines := []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"make build","description":"Run build"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/workspace/main.go"}}]}}`,
		`{"type":"system","subtype":"task_progress","description":"Building project"}`,
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(logLines, "\n")), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// Create a running session
	sess, err := state.Create(repo, state.CreateOptions{
		WorktreePath: repo + "-test-sandbox",
		Branch:       "sandbox/test",
		Name:         "running-test",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess.Status = state.StatusRunning
	sess.StartedAt = time.Now().Add(-5 * time.Minute)
	sess.LogPath = logPath
	if err := state.Update(repo, sess); err != nil {
		t.Fatalf("update session: %v", err)
	}

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"status", "--session", "running-test"})

	// Should not crash — that's the main assertion
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status should not crash for running session: %v", err)
	}

	output := buf.String()
	// Should show session info
	if !strings.Contains(output, "running") {
		t.Errorf("expected 'running' in output, got: %s", output)
	}
	// Should show some progress info (either analysis or fallback)
	if !strings.Contains(output, "tool calls") && !strings.Contains(output, "progress") && !strings.Contains(output, "Analyzing") && !strings.Contains(output, "Execution in progress") {
		t.Errorf("expected progress info in output, got: %s", output)
	}
}
```

- [ ] **Step 2: Run test to verify current behavior**

Run: `go test ./internal/cli/ -run TestStatusCommand_RunningWithLog -v`
Expected: May pass or fail depending on claude CLI availability — either way, no crash

- [ ] **Step 3: Update status.go StatusRunning case**

Replace the `case state.StatusRunning:` block in `internal/cli/status.go` (lines 71-113) with:

```go
	case state.StatusRunning:
		summary, err := parseLogEvents(sess.LogPath, sess.StartedAt)
		if err != nil || summary.TotalTools == 0 {
			cmd.Println("Execution in progress. No log data yet.")
			return nil
		}

		if !claudeAvailable() {
			cmd.Println(formatFallback(summary, "claude CLI not found"))
			return nil
		}

		// Show spinner while analyzing
		spinChars := []string{"|", "/", "-", "\\"}
		formattedSummary := formatSummary(summary)
		done := make(chan struct{ analysis, reason string }, 1)
		ctx := cmd.Context()
		go func() {
			analysis, reason := analyzeLog(ctx, formattedSummary)
			done <- struct{ analysis, reason string }{analysis, reason}
		}()

		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Print("\r\033[K")
				return ctx.Err()
			case result := <-done:
				fmt.Print("\r\033[K")
				if result.reason != "" {
					cmd.Println(formatFallback(summary, result.reason))
				} else {
					cmd.Println(result.analysis)
				}
				return nil
			case <-ticker.C:
				fmt.Printf("\r%s Analyzing...", spinChars[i%len(spinChars)])
				i++
			}
		}
```

- [ ] **Step 4: Run all status tests**

Run: `go test ./internal/cli/ -run TestStatus -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/status.go internal/cli/status_test.go
git commit -m "fix(status): use parsed log summary instead of raw content for analysis"
```

---

### Task 7: Delete logutil.go and logutil_test.go

**Files:**
- Delete: `internal/cli/logutil.go`
- Delete: `internal/cli/logutil_test.go`

- [ ] **Step 1: Verify readLogTail is no longer referenced**

Run: `grep -r "readLogTail" internal/`
Expected: Only hits in `logutil.go` and `logutil_test.go` (no references from status.go anymore)

- [ ] **Step 2: Delete the files**

```bash
rm internal/cli/logutil.go internal/cli/logutil_test.go
```

- [ ] **Step 3: Run all tests to verify nothing breaks**

Run: `go test ./... && go build ./...`
Expected: All PASS, build succeeds

- [ ] **Step 4: Commit**

```bash
git add internal/cli/logutil.go internal/cli/logutil_test.go
git commit -m "refactor(status): remove readLogTail replaced by parseLogEvents"
```

---

### Task 8: Run full quality gates

**Files:** None (verification only)

- [ ] **Step 1: Build**

Run: `make build`
Expected: PASS

- [ ] **Step 2: Lint**

Run: `golangci-lint run ./...`
Expected: 0 issues

- [ ] **Step 3: Tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 4: Tidy**

Run: `go mod tidy && git diff --exit-code go.mod go.sum`
Expected: No changes

- [ ] **Step 5: Fix any issues found, commit if needed**

---

### Task 9: Create E2E workflow test script

**Files:**
- Create: `test/e2e/workflow_test.sh`

- [ ] **Step 1: Create the test script**

```bash
#!/usr/bin/env bash
set -Eeuo pipefail

# E2E workflow test: spec → execute → status → clean
# Requires: claude-sandbox binary in PATH, ANTHROPIC_API_KEY set, Docker running
#
# ship is excluded — creates real PRs, would pollute repos with test garbage

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SESSION_NAME="e2e-test-$$"  # PID suffix to avoid collision
REPO_ROOT=""

cleanup() {
    local exit_code=$?
    echo ""
    if [[ -n "$REPO_ROOT" ]]; then
        echo "Cleaning up..."
        claude-sandbox clean --session "$SESSION_NAME" 2>/dev/null || true
    fi
    if [[ $exit_code -eq 0 ]]; then
        echo "✓ E2E workflow passed"
    else
        echo "✗ E2E workflow failed (exit code: $exit_code)"
    fi
    exit $exit_code
}
trap cleanup EXIT

# Ensure we're in a git repo
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "=== E2E Workflow Test ==="
echo "Session: $SESSION_NAME"
echo ""

# Pre-flight checks
command -v claude-sandbox >/dev/null 2>&1 || { echo "Error: claude-sandbox not in PATH" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Error: docker not found" >&2; exit 1; }
[[ -n "${ANTHROPIC_API_KEY:-}" ]] || { echo "Error: ANTHROPIC_API_KEY not set" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Error: docker daemon not running" >&2; exit 1; }

# 1. Create a session with a simple PLAN.md
echo "--- Step 1: Creating session with PLAN.md ---"
# spec runs interactively, so we create the worktree + session manually
# and write PLAN.md directly for deterministic testing
SESSION_DIR="$REPO_ROOT/.claude-sandbox/sessions"
claude-sandbox spec --name "$SESSION_NAME" </dev/null || true

# Write a trivial PLAN.md into the worktree
WORKTREE_PATH=$(jq -r .worktree_path "$SESSION_DIR/$SESSION_NAME.json" 2>/dev/null)
if [[ -z "$WORKTREE_PATH" || "$WORKTREE_PATH" == "null" ]]; then
    # Find by listing sessions
    echo "Error: could not find session worktree path" >&2
    exit 1
fi
cat > "$WORKTREE_PATH/PLAN.md" << 'PLAN'
# Test Plan

Create a file `hello.txt` containing "Hello World".
Commit the file.
Write COMPLETION.md with Status: SUCCESS.
PLAN
echo "PLAN.md written to $WORKTREE_PATH"

# 2. Execute
echo ""
echo "--- Step 2: Execute ---"
timeout 180 claude-sandbox execute --session "$SESSION_NAME"
EXECUTE_EXIT=$?
if [[ $EXECUTE_EXIT -ne 0 ]]; then
    echo "Warning: execute exited with code $EXECUTE_EXIT"
fi

# 3. Status (should show completed state, not crash)
echo ""
echo "--- Step 3: Status ---"
STATUS_OUTPUT=$(claude-sandbox status --session "$SESSION_NAME" 2>&1)
echo "$STATUS_OUTPUT"

# Verify status shows completion (success or failed — either is valid, crash is not)
if echo "$STATUS_OUTPUT" | grep -qE "(completed successfully|Failed|Blocked)"; then
    echo "✓ Status command works for completed session"
else
    echo "✗ Status output unexpected" >&2
    exit 1
fi

# 4. Clean
echo ""
echo "--- Step 4: Clean ---"
claude-sandbox clean --session "$SESSION_NAME" --all
echo "✓ Clean completed"
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x test/e2e/workflow_test.sh
```

- [ ] **Step 3: Commit**

```bash
git add test/e2e/workflow_test.sh
git commit -m "test(e2e): add workflow test for spec → execute → status → clean"
```

---

### Task 10: Record session log for playback testing

**Files:**
- Create: `testdata/session-log-sample.json`
- Modify: `internal/cli/logsummary_test.go`

- [ ] **Step 1: Create a minimal but realistic recorded session log**

```bash
mkdir -p testdata
```

Write `testdata/session-log-sample.json` with a representative subset of events:

```json
{"type":"system","subtype":"init","session_id":"test-session"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls /workspace && cat /workspace/PLAN.md","description":"Check workspace contents and PLAN.md"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"main.go\nPLAN.md\n# Test Plan\n..."}]},"timestamp":"2026-03-28T03:31:38.037Z"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"make build 2>&1 && echo BUILD PASS","description":"Run build"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"BUILD PASS"}]},"timestamp":"2026-03-28T03:32:00.000Z"}
{"type":"system","subtype":"task_progress","description":"Running build checks"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/workspace/internal/cli/status.go"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"package cli\n..."}]},"timestamp":"2026-03-28T03:32:10.000Z"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/workspace/internal/cli/status.go","old_string":"old","new_string":"new"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"File updated"}]},"timestamp":"2026-03-28T03:32:20.000Z"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./... 2>&1","description":"Run tests"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"ok  \tgithub.com/example\t0.5s"}]},"timestamp":"2026-03-28T03:33:00.000Z"}
{"type":"system","subtype":"task_progress","description":"Running tests"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"golangci-lint run ./... 2>&1 && echo LINT PASS","description":"Run lint"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"LINT PASS"}]},"timestamp":"2026-03-28T03:33:30.000Z"}
{"type":"system","subtype":"task_progress","description":"Checking lint results"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task","input":{"description":"Run /review-code"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"Grade A (135/140)"}]},"timestamp":"2026-03-28T03:35:00.000Z"}
{"type":"result","subtype":"success","duration_ms":210000,"result":"Implementation complete."}
```

- [ ] **Step 2: Write test using recorded session**

Add to `internal/cli/logsummary_test.go`:

```go
func TestParseLogEvents_RecordedSession(t *testing.T) {
	// Use the recorded session log from testdata/
	path := filepath.Join("..", "..", "testdata", "session-log-sample.json")

	// Check file exists (catches missing testdata)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("testdata/session-log-sample.json not found")
	}

	summary, err := parseLogEvents(path, time.Date(2026, 3, 28, 3, 31, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tool counts match the recorded session
	if summary.TotalTools != 7 {
		t.Errorf("TotalTools = %d, want 7", summary.TotalTools)
	}
	if summary.ToolCounts["Bash"] != 4 {
		t.Errorf("Bash count = %d, want 4", summary.ToolCounts["Bash"])
	}
	if summary.ToolCounts["Read"] != 1 {
		t.Errorf("Read count = %d, want 1", summary.ToolCounts["Read"])
	}
	if summary.ToolCounts["Edit"] != 1 {
		t.Errorf("Edit count = %d, want 1", summary.ToolCounts["Edit"])
	}
	if summary.ToolCounts["Task"] != 1 {
		t.Errorf("Task count = %d, want 1", summary.ToolCounts["Task"])
	}

	// Verify gate detection
	for _, gate := range []string{"build", "lint", "test"} {
		if !summary.GateMentions[gate] {
			t.Errorf("gate %q not detected", gate)
		}
	}
	// review-code is in Task input description, not Bash — should NOT be detected
	// (gates only check Bash commands per spec)
	if summary.GateMentions["review-code"] {
		t.Error("review-code should not be detected from Task tool")
	}

	// Verify task progress
	if len(summary.TaskProgress) != 3 {
		t.Errorf("TaskProgress length = %d, want 3", len(summary.TaskProgress))
	}

	// Verify formatSummary doesn't crash
	result := formatSummary(summary)
	if result == "" {
		t.Error("formatSummary returned empty string")
	}
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/cli/ -run TestParseLogEvents_RecordedSession -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add testdata/session-log-sample.json internal/cli/logsummary_test.go
git commit -m "test(status): add recorded session playback test"
```

---

### Task 11: Final quality gates and cleanup

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 2: Run lint**

Run: `golangci-lint run ./...`
Expected: 0 issues

- [ ] **Step 3: Run build**

Run: `make build`
Expected: PASS

- [ ] **Step 4: Run tidy**

Run: `go mod tidy && git diff --exit-code go.mod go.sum`
Expected: No changes

- [ ] **Step 5: Manual smoke test (if session available)**

Run: `claude-sandbox status` against a real or mock session to verify the output looks correct.
