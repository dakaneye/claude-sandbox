# Status Command Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enhance `claude-sandbox status` to provide AI-powered progress analysis by capturing container output to a log file and using Claude haiku to summarize progress.

**Architecture:** Two changes: (1) `run` command captures container stdout/stderr to log file via `io.MultiWriter`, (2) `status` command reads log tail and calls `claude -p --model haiku` for analysis. Graceful fallback if log missing or haiku fails.

**Tech Stack:** Go, exec.Command for Claude CLI, io.MultiWriter for log capture

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/container/container.go` | Add `LogWriter` field to `RunOptions`, write to it if provided |
| `internal/cli/run.go` | Create log file, pass writer to container.Run |
| `internal/cli/status.go` | Read log tail, call haiku for analysis, display summary |

---

### Task 1: Add LogWriter to RunOptions

**Files:**
- Modify: `internal/container/container.go:24-31` (RunOptions struct)
- Modify: `internal/container/container.go:103-106` (Run function stdout/stderr handling)

- [ ] **Step 1: Add LogWriter field to RunOptions**

In `internal/container/container.go`, add the `io` import and `LogWriter` field:

```go
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)
```

```go
// RunOptions configures container execution.
type RunOptions struct {
	Image        string
	WorktreePath string
	HomeDir      string
	SpecPath     string
	Interactive  bool
	LogWriter    io.Writer // If set, container output is written here
}
```

- [ ] **Step 2: Update Run function to use LogWriter**

In the `Run` function, change stdout/stderr handling:

```go
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin
	if opts.LogWriter != nil {
		cmd.Stdout = opts.LogWriter
		cmd.Stderr = opts.LogWriter
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
```

- [ ] **Step 3: Run tests to verify no regression**

Run: `go test -race ./internal/container/...`
Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/container/container.go
git commit -m "feat(container): add LogWriter option to capture output"
```

---

### Task 2: Capture container output to log file in run command

**Files:**
- Modify: `internal/cli/run.go:115-125` (container.Run call)

- [ ] **Step 1: Create log file before running container**

In `run.go`, after the session is created, open the log file. Update imports first:

```go
import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
	"github.com/dakaneye/claude-sandbox/internal/session"
)
```

Then in `runRun`, after the session header prints, add log file creation:

```go
	cmd.Println()

	// Create log file for capturing container output
	logFile, err := os.Create(sess.LogPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
```

- [ ] **Step 2: Pass LogWriter to container.Run**

Update the container.Run call to pass the log file:

```go
	go func() {
		done <- container.Run(ctx, container.RunOptions{
			Image:        container.DefaultImage,
			WorktreePath: wt.Path,
			HomeDir:      home,
			SpecPath:     absSpec,
			Interactive:  false,
			LogWriter:    logFile,
		})
	}()
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./internal/cli/...`
Expected: All tests pass

- [ ] **Step 4: Build and verify manually**

Run: `go build ./cmd/claude-sandbox && ./claude-sandbox --help`
Expected: Builds without errors

- [ ] **Step 5: Commit**

```bash
git add internal/cli/run.go
git commit -m "feat(cli): capture container output to log file"
```

---

### Task 3: Add log reading helper

**Files:**
- Create: `internal/cli/logutil.go`

- [ ] **Step 1: Create logutil.go with readLogTail function**

```go
package cli

import (
	"bufio"
	"os"
	"strings"
)

// readLogTail reads the last n lines from a file.
// Returns empty string if file doesn't exist or is empty.
func readLogTail(path string, lines int) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read all lines into a ring buffer of size n
	ring := make([]string, lines)
	index := 0
	count := 0

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		ring[index] = scanner.Text()
		index = (index + 1) % lines
		count++
	}

	if count == 0 {
		return ""
	}

	// Build result from ring buffer in correct order
	var result []string
	if count < lines {
		result = ring[:count]
	} else {
		// Ring buffer wrapped, start from index
		result = make([]string, lines)
		for i := 0; i < lines; i++ {
			result[i] = ring[(index+i)%lines]
		}
	}

	return strings.Join(result, "\n")
}
```

- [ ] **Step 2: Run tests to verify compilation**

Run: `go build ./internal/cli/...`
Expected: Builds without errors

- [ ] **Step 3: Commit**

```bash
git add internal/cli/logutil.go
git commit -m "feat(cli): add log tail reading utility"
```

---

### Task 4: Add Claude analysis helper

**Files:**
- Create: `internal/cli/analyze.go`

- [ ] **Step 1: Create analyze.go with analyzeLog function**

```go
package cli

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const analysisPrompt = `Analyze this Claude Code session log. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's currently doing
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Log tail:
`

// analyzeLog uses Claude haiku to analyze the log content.
// Returns empty string if analysis fails (graceful degradation).
func analyzeLog(logContent string) string {
	if logContent == "" {
		return ""
	}

	prompt := analysisPrompt + logContent

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", prompt)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// claudeAvailable checks if the claude CLI is available.
func claudeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}
```

- [ ] **Step 2: Run tests to verify compilation**

Run: `go build ./internal/cli/...`
Expected: Builds without errors

- [ ] **Step 3: Commit**

```bash
git add internal/cli/analyze.go
git commit -m "feat(cli): add Claude log analysis helper"
```

---

### Task 5: Enhance status command with analysis

**Files:**
- Modify: `internal/cli/status.go`

- [ ] **Step 1: Update imports**

```go
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/session"
)
```

- [ ] **Step 2: Rewrite runStatus with analysis**

```go
func runStatus(cmd *cobra.Command, args []string) error {
	wt, err := requireWorktree()
	if err != nil {
		return err
	}

	sess, err := session.Load(wt.Path)
	if err != nil {
		cmd.Println("No session found in this worktree.")
		return nil
	}

	// Basic session info
	cmd.Printf("Session: %s\n", sess.ID)
	cmd.Printf("Status:  %s\n", sess.Status)
	cmd.Printf("Elapsed: %s\n", sess.Duration().Round(time.Second))
	cmd.Println()

	// If completed, skip analysis
	if sess.Status != session.StatusRunning {
		if sess.Status == session.StatusSuccess {
			cmd.Println("✓ Session completed successfully. See COMPLETION.md")
		} else {
			cmd.Printf("✗ Session failed: %s\n", sess.Error)
		}
		return nil
	}

	// Read log and analyze
	logContent := readLogTail(sess.LogPath, 500)
	if logContent == "" {
		cmd.Println("Log not available yet")
		return nil
	}

	if !claudeAvailable() {
		cmd.Println("(Claude CLI not available for analysis)")
		return nil
	}

	// Show spinner while analyzing
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan string, 1)
	go func() {
		done <- analyzeLog(logContent)
	}()

	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case analysis := <-done:
			fmt.Print("\r\033[K") // Clear spinner
			if analysis == "" {
				cmd.Println("Could not analyze log")
			} else {
				cmd.Println(analysis)
			}
			return nil
		case <-ticker.C:
			fmt.Printf("\r%s Analyzing...", spinChars[i%len(spinChars)])
			i++
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race ./internal/cli/...`
Expected: All tests pass

- [ ] **Step 4: Build and test manually**

Run: `go build -o bin/claude-sandbox ./cmd/claude-sandbox`
Expected: Builds without errors

- [ ] **Step 5: Commit**

```bash
git add internal/cli/status.go
git commit -m "feat(cli): add AI-powered progress analysis to status command"
```

---

### Task 6: Integration test and final verification

**Files:**
- None (manual testing)

- [ ] **Step 1: Run all tests**

Run: `go test -race ./...`
Expected: All tests pass

- [ ] **Step 2: Run linter**

Run: `golangci-lint run ./...`
Expected: No issues

- [ ] **Step 3: Install and verify**

Run: `make install && claude-sandbox --help`
Expected: Installs and shows help

- [ ] **Step 4: Final commit if any cleanup needed**

```bash
git status
# If changes needed, commit them
```

- [ ] **Step 5: Push changes**

```bash
git push
```
