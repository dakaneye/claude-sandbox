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
