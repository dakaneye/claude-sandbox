# Main-Repo-Centric CLI Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor claude-sandbox CLI so all commands run from the main repository, with state tracked in `.claude-sandbox/` directory.

**Architecture:** New `internal/state` package manages session state in `.claude-sandbox/sessions/`. CLI commands use `resolveSession()` helper instead of `requireWorktree()`. Commands renamed: `init`→`spec`, `run`→`execute`. Remove `logs` command.

**Tech Stack:** Go, Cobra CLI, JSON for state persistence

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/state/state.go` | Session state management (Create, Get, List, Remove, etc.) |
| `internal/state/state_test.go` | Tests for state package |
| `internal/state/picker.go` | Interactive session picker for multiple sessions |
| `internal/cli/spec.go` | `spec` command (replaces init.go) |
| `internal/cli/execute.go` | `execute` command (replaces run.go) |
| `internal/cli/status.go` | Updated to use state resolution |
| `internal/cli/stop.go` | Updated to use state resolution |
| `internal/cli/ship.go` | Updated to use state resolution |
| `internal/cli/clean.go` | Rewritten to use state package |
| `internal/cli/helpers.go` | Replace `requireWorktree()` with `resolveSession()` |
| `internal/cli/root.go` | Update command registration |

**Files to delete:**
- `internal/cli/init.go`
- `internal/cli/run.go`
- `internal/cli/logs.go`
- `internal/session/session.go` (functionality moves to state)

---

### Task 1: Create state package with Session type and basic operations

**Files:**
- Create: `internal/state/state.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write failing test for EnsureDir**

```go
// internal/state/state_test.go
package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDir(t *testing.T) {
	repoPath := t.TempDir()

	err := EnsureDir(repoPath)
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	sessionsDir := filepath.Join(repoPath, ".claude-sandbox", "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Errorf("sessions directory not created: %s", sessionsDir)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/state/...`
Expected: FAIL (package doesn't exist)

- [ ] **Step 3: Create state.go with Status type and EnsureDir**

```go
// internal/state/state.go
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dakaneye/claude-sandbox/internal/id"
)

const (
	stateDir     = ".claude-sandbox"
	sessionsDir  = "sessions"
	activeFile   = "active"
	logDirPath   = ".claude/sandbox-sessions"
)

// Status represents the session state.
type Status string

const (
	StatusSpeccing Status = "speccing"
	StatusReady    Status = "ready"
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusBlocked  Status = "blocked"
)

// Session represents a sandbox session.
type Session struct {
	ID           string    `json:"id"`
	Name         string    `json:"name,omitempty"`
	WorktreePath string    `json:"worktree_path"`
	Branch       string    `json:"branch"`
	Status       Status    `json:"status"`
	LogPath      string    `json:"log_path"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// EnsureDir creates .claude-sandbox/sessions/ if needed.
func EnsureDir(repoPath string) error {
	dir := filepath.Join(repoPath, stateDir, sessionsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}
	return nil
}

// sessionsPath returns the path to the sessions directory.
func sessionsPath(repoPath string) string {
	return filepath.Join(repoPath, stateDir, sessionsDir)
}

// activePath returns the path to the active file.
func activePath(repoPath string) string {
	return filepath.Join(repoPath, stateDir, activeFile)
}

// sessionPath returns the path to a session file.
func sessionPath(repoPath, id string) string {
	return filepath.Join(sessionsPath(repoPath), id+".json")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/state/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): add state package with EnsureDir"
```

---

### Task 2: Add Create and Get session operations

**Files:**
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`

- [ ] **Step 1: Write failing test for Create**

```go
// Add to internal/state/state_test.go
func TestCreate(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sess.ID == "" {
		t.Error("session ID is empty")
	}
	if sess.Status != StatusSpeccing {
		t.Errorf("expected status %q, got %q", StatusSpeccing, sess.Status)
	}
	if sess.WorktreePath != "/path/to/worktree" {
		t.Errorf("expected worktree path %q, got %q", "/path/to/worktree", sess.WorktreePath)
	}

	// Verify file was created
	path := sessionPath(repoPath, sess.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("session file not created: %s", path)
	}
}

func TestCreateWithName(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
		Name:         "feature-x",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sess.Name != "feature-x" {
		t.Errorf("expected name %q, got %q", "feature-x", sess.Name)
	}

	// Verify symlink was created
	linkPath := filepath.Join(sessionsPath(repoPath), "feature-x.json")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if target != sess.ID+".json" {
		t.Errorf("symlink target %q, expected %q", target, sess.ID+".json")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/state/...`
Expected: FAIL (Create not defined)

- [ ] **Step 3: Implement Create**

```go
// Add to internal/state/state.go

// CreateOptions configures session creation.
type CreateOptions struct {
	WorktreePath string
	Branch       string
	Name         string
}

// Create creates a new session and sets it as active.
func Create(repoPath string, opts CreateOptions) (*Session, error) {
	if err := EnsureDir(repoPath); err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	sessionID := id.NewSessionID()
	sess := &Session{
		ID:           sessionID,
		Name:         opts.Name,
		WorktreePath: opts.WorktreePath,
		Branch:       opts.Branch,
		Status:       StatusSpeccing,
		LogPath:      filepath.Join(home, logDirPath, sessionID+".log"),
		CreatedAt:    time.Now(),
	}

	// Save session file
	if err := saveSession(repoPath, sess); err != nil {
		return nil, err
	}

	// Create name symlink if provided
	if opts.Name != "" {
		linkPath := filepath.Join(sessionsPath(repoPath), opts.Name+".json")
		// Remove existing symlink if present
		os.Remove(linkPath)
		if err := os.Symlink(sessionID+".json", linkPath); err != nil {
			return nil, fmt.Errorf("create name symlink: %w", err)
		}
	}

	// Set as active
	if err := SetActive(repoPath, sessionID); err != nil {
		return nil, err
	}

	return sess, nil
}

// saveSession writes session to disk.
func saveSession(repoPath string, sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := sessionPath(repoPath, sess.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// SetActive updates the active session pointer.
func SetActive(repoPath, id string) error {
	path := activePath(repoPath)
	if err := os.WriteFile(path, []byte(id), 0644); err != nil {
		return fmt.Errorf("write active file: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/state/...`
Expected: PASS

- [ ] **Step 5: Write failing test for Get**

```go
// Add to internal/state/state_test.go
func TestGet(t *testing.T) {
	repoPath := t.TempDir()

	created, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
		Name:         "feature-x",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by ID
	sess, err := Get(repoPath, created.ID)
	if err != nil {
		t.Fatalf("Get by ID failed: %v", err)
	}
	if sess.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, sess.ID)
	}

	// Get by name
	sess, err = Get(repoPath, "feature-x")
	if err != nil {
		t.Fatalf("Get by name failed: %v", err)
	}
	if sess.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, sess.ID)
	}
}

func TestGetNotFound(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	_, err := Get(repoPath, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test -v ./internal/state/...`
Expected: FAIL (Get not defined)

- [ ] **Step 7: Implement Get**

```go
// Add to internal/state/state.go

// Get loads a session by ID or name.
func Get(repoPath, idOrName string) (*Session, error) {
	// Try direct ID first
	path := sessionPath(repoPath, idOrName)
	data, err := os.ReadFile(path)
	if err == nil {
		return parseSession(data)
	}

	// Try as name (symlink)
	linkPath := filepath.Join(sessionsPath(repoPath), idOrName+".json")
	data, err = os.ReadFile(linkPath)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", idOrName)
	}

	return parseSession(data)
}

func parseSession(data []byte) (*Session, error) {
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &sess, nil
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test -v ./internal/state/...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): add Create and Get operations"
```

---

### Task 3: Add List, Update, and Remove operations

**Files:**
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`

- [ ] **Step 1: Write failing test for List**

```go
// Add to internal/state/state_test.go
func TestList(t *testing.T) {
	repoPath := t.TempDir()

	// Create multiple sessions
	_, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/wt1",
		Branch:       "sandbox/2026-03-27-aaa111",
	})
	if err != nil {
		t.Fatalf("Create 1 failed: %v", err)
	}

	_, err = Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/wt2",
		Branch:       "sandbox/2026-03-27-bbb222",
		Name:         "feature-y",
	})
	if err != nil {
		t.Fatalf("Create 2 failed: %v", err)
	}

	sessions, err := List(repoPath)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestListEmpty(t *testing.T) {
	repoPath := t.TempDir()
	_ = EnsureDir(repoPath)

	sessions, err := List(repoPath)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/state/...`
Expected: FAIL (List not defined)

- [ ] **Step 3: Implement List**

```go
// Add to internal/state/state.go

// List returns all sessions, sorted by creation time (newest first).
func List(repoPath string) ([]*Session, error) {
	dir := sessionsPath(repoPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	var sessions []*Session
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		// Skip symlinks (names), we'll get the actual file
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		id := strings.TrimSuffix(name, ".json")
		if seen[id] {
			continue
		}
		seen[id] = true

		sess, err := Get(repoPath, id)
		if err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}

	// Sort by creation time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}
```

Add `"sort"` and `"strings"` to imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/state/...`
Expected: PASS

- [ ] **Step 5: Write failing test for Update**

```go
// Add to internal/state/state_test.go
func TestUpdate(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	sess.Status = StatusReady
	if err := Update(repoPath, sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Reload and verify
	reloaded, err := Get(repoPath, sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if reloaded.Status != StatusReady {
		t.Errorf("expected status %q, got %q", StatusReady, reloaded.Status)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test -v ./internal/state/...`
Expected: FAIL (Update not defined)

- [ ] **Step 7: Implement Update**

```go
// Add to internal/state/state.go

// Update saves changes to an existing session.
func Update(repoPath string, sess *Session) error {
	return saveSession(repoPath, sess)
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test -v ./internal/state/...`
Expected: PASS

- [ ] **Step 9: Write failing test for Remove**

```go
// Add to internal/state/state_test.go
func TestRemove(t *testing.T) {
	repoPath := t.TempDir()

	sess, err := Create(repoPath, CreateOptions{
		WorktreePath: "/path/to/worktree",
		Branch:       "sandbox/2026-03-27-abc123",
		Name:         "feature-x",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := Remove(repoPath, sess.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify session file is gone
	path := sessionPath(repoPath, sess.ID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("session file still exists")
	}

	// Verify symlink is gone
	linkPath := filepath.Join(sessionsPath(repoPath), "feature-x.json")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("name symlink still exists")
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `go test -v ./internal/state/...`
Expected: FAIL (Remove not defined)

- [ ] **Step 11: Implement Remove**

```go
// Add to internal/state/state.go

// Remove deletes a session and its name symlink.
func Remove(repoPath, id string) error {
	// Load session to get name
	sess, err := Get(repoPath, id)
	if err != nil {
		return err
	}

	// Remove session file
	path := sessionPath(repoPath, sess.ID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}

	// Remove name symlink if exists
	if sess.Name != "" {
		linkPath := filepath.Join(sessionsPath(repoPath), sess.Name+".json")
		os.Remove(linkPath) // Ignore errors
	}

	// Clear active if this was the active session
	active, _ := GetActiveID(repoPath)
	if active == sess.ID {
		os.Remove(activePath(repoPath))
	}

	return nil
}

// GetActiveID returns the ID of the active session.
func GetActiveID(repoPath string) (string, error) {
	data, err := os.ReadFile(activePath(repoPath))
	if err != nil {
		return "", fmt.Errorf("read active file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `go test -v ./internal/state/...`
Expected: PASS

- [ ] **Step 13: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat(state): add List, Update, and Remove operations"
```

---

### Task 4: Add interactive session picker

**Files:**
- Create: `internal/state/picker.go`
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`

- [ ] **Step 1: Create picker.go with PickSession**

```go
// internal/state/picker.go
package state

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// PickSession displays an interactive picker for multiple sessions.
// Returns the selected session or error if cancelled.
func PickSession(sessions []*Session) (*Session, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions available")
	}

	fmt.Println("Multiple sessions found. Select one:")
	fmt.Println()

	for i, sess := range sessions {
		name := sess.ID
		if sess.Name != "" {
			name = fmt.Sprintf("%s (%s)", sess.Name, sess.ID)
		}
		age := time.Since(sess.CreatedAt).Round(time.Minute)
		fmt.Printf("  [%d] %s - %s (%s ago)\n", i+1, name, sess.Status, age)
	}

	fmt.Println()
	fmt.Print("Enter number (or 'q' to cancel): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "q" || input == "" {
		return nil, fmt.Errorf("cancelled")
	}

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(sessions) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return sessions[num-1], nil
}
```

- [ ] **Step 2: Add ResolveSession to state.go**

```go
// Add to internal/state/state.go

// ResolveSession resolves a session from ID/name or interactively.
// If idOrName is empty, uses the active session or picker if multiple.
func ResolveSession(repoPath, idOrName string) (*Session, error) {
	// Explicit ID/name provided
	if idOrName != "" {
		return Get(repoPath, idOrName)
	}

	sessions, err := List(repoPath)
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found. Run 'claude-sandbox spec' first")
	}

	if len(sessions) == 1 {
		return sessions[0], nil
	}

	// Multiple sessions - use picker
	return PickSession(sessions)
}
```

- [ ] **Step 3: Run tests to verify compilation**

Run: `go build ./internal/state/...`
Expected: Builds without errors

- [ ] **Step 4: Commit**

```bash
git add internal/state/picker.go internal/state/state.go
git commit -m "feat(state): add interactive session picker"
```

---

### Task 5: Add resolveSession helper to CLI

**Files:**
- Modify: `internal/cli/helpers.go`

- [ ] **Step 1: Read current helpers.go**

Review the existing `requireWorktree()` and `promptYesNo()` functions.

- [ ] **Step 2: Replace requireWorktree with resolveSession**

```go
// internal/cli/helpers.go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

// resolveSession resolves a session from the --session flag or interactively.
func resolveSession(cmd *cobra.Command, sessionFlag string) (*state.Session, error) {
	repoPath, err := findRepoRoot()
	if err != nil {
		return nil, err
	}

	return state.ResolveSession(repoPath, sessionFlag)
}

// findRepoRoot finds the root of the git repository from cwd.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	if !worktree.IsGitRepo(cwd) {
		return "", fmt.Errorf("not a git repository")
	}

	// Get the toplevel
	wt, err := worktree.Detect(cwd)
	if err != nil {
		return "", fmt.Errorf("detect git root: %w", err)
	}

	return wt.Path, nil
}

// promptYesNo prompts the user for a yes/no confirmation.
func promptYesNo(cmd *cobra.Command, question string, defaultYes bool) bool {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}

	cmd.Printf("%s %s ", question, suffix)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./internal/cli/...`
Expected: Builds (may have unused import warnings until commands are updated)

- [ ] **Step 4: Commit**

```bash
git add internal/cli/helpers.go
git commit -m "refactor(cli): replace requireWorktree with resolveSession"
```

---

### Task 6: Create spec command (replace init)

**Files:**
- Create: `internal/cli/spec.go`
- Delete: `internal/cli/init.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Create spec.go**

```go
// internal/cli/spec.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

func newSpecCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Create a new sandbox session and launch Claude for spec-driven planning",
		Long: `Creates a new sandbox session with an isolated git worktree, then launches
an interactive Claude session to create SPEC.md and PLAN.md.

The workflow:
  1. Creates a git worktree for isolated work
  2. Launches Claude with guidance to use brainstorming and planning skills
  3. You collaborate with Claude to create SPEC.md and PLAN.md
  4. When Claude exits, the session is ready for 'claude-sandbox execute'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSpec(cmd, name)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Friendly name for the session")

	return cmd
}

func runSpec(cmd *cobra.Command, name string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	cmd.Println("Creating sandbox session...")

	// Create worktree
	wt, err := worktree.Create(repoPath)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	// Create session
	sess, err := state.Create(repoPath, state.CreateOptions{
		WorktreePath: wt.Path,
		Branch:       wt.Branch,
		Name:         name,
	})
	if err != nil {
		// Clean up worktree on failure
		worktree.Remove(wt.Path)
		return fmt.Errorf("create session: %w", err)
	}

	cmd.Printf("  Session: %s\n", sess.ID)
	if name != "" {
		cmd.Printf("  Name:    %s\n", name)
	}
	cmd.Printf("  Branch:  %s\n", wt.Branch)
	cmd.Printf("  Path:    %s\n", wt.Path)
	cmd.Println()
	cmd.Println("Launching Claude for spec creation...")
	cmd.Println("Use /superpowers:brainstorming to create SPEC.md, then /superpowers:writing-plans to create PLAN.md")
	cmd.Println()

	// Launch Claude in worktree
	claudeCmd := exec.Command("claude")
	claudeCmd.Dir = wt.Path
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr

	claudeErr := claudeCmd.Run()

	// Check if PLAN.md was created
	planPath := filepath.Join(wt.Path, "PLAN.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		sess.Status = state.StatusFailed
		sess.Error = "Spec incomplete: PLAN.md not created"
		state.Update(repoPath, sess)
		cmd.PrintErrf("\nWarning: PLAN.md not found. Session marked as failed.\n")
		cmd.PrintErrf("Run 'claude-sandbox spec' to try again, or manually create PLAN.md.\n")
		return claudeErr
	}

	sess.Status = state.StatusReady
	if err := state.Update(repoPath, sess); err != nil {
		cmd.PrintErrf("Warning: failed to update session status: %v\n", err)
	}

	cmd.Println()
	cmd.Println("Spec complete! Next step:")
	cmd.Println("  claude-sandbox execute")

	return nil
}
```

- [ ] **Step 2: Delete init.go**

```bash
rm internal/cli/init.go
```

- [ ] **Step 3: Update root.go**

```go
// internal/cli/root.go
package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claude-sandbox",
		Short:   "Sandboxed execution environment for Claude Code",
		Long:    `claude-sandbox enables autonomous Claude Code execution in isolated containers with quality gates and external action blocking.`,
		Version: version,
	}

	cmd.AddCommand(newSpecCommand())
	cmd.AddCommand(newExecuteCommand())
	cmd.AddCommand(newShipCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newStopCommand())
	cmd.AddCommand(newCleanCommand())

	return cmd
}
```

- [ ] **Step 4: Run build to verify**

Run: `go build ./...`
Expected: Build errors (newExecuteCommand not defined yet)

- [ ] **Step 5: Commit spec.go and root.go changes**

```bash
git add internal/cli/spec.go internal/cli/root.go
git rm internal/cli/init.go
git commit -m "feat(cli): add spec command, remove init"
```

---

### Task 7: Create execute command (replace run)

**Files:**
- Create: `internal/cli/execute.go`
- Delete: `internal/cli/run.go`

- [ ] **Step 1: Create execute.go**

```go
// internal/cli/execute.go
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newExecuteCommand() *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute the plan in a sandboxed container",
		Long: `Executes the implementation plan (PLAN.md) in an isolated container.

Claude implements the plan with quality gates:
  - Build must succeed
  - Lint must pass
  - Tests must pass
  - /review-code must return grade A

COMPLETION.md is written when done (success or blocked).

Execution is idempotent: if COMPLETION.md shows FAILED/BLOCKED,
running execute again will continue from where it left off.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecute(cmd, sessionFlag)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name")

	return cmd
}

func runExecute(cmd *cobra.Command, sessionFlag string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	// Check COMPLETION.md status
	completionPath := filepath.Join(sess.WorktreePath, "COMPLETION.md")
	if content, err := os.ReadFile(completionPath); err == nil {
		if strings.Contains(string(content), "Status: SUCCESS") {
			cmd.Println("Session already completed successfully. Nothing to do.")
			cmd.Println("Use 'claude-sandbox ship' to create a PR.")
			return nil
		}
		// FAILED or BLOCKED - continue execution
		cmd.Println("Previous execution incomplete. Continuing...")
	}

	// Verify PLAN.md exists
	planPath := filepath.Join(sess.WorktreePath, "PLAN.md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("PLAN.md not found in worktree. Run 'claude-sandbox spec' first")
	}

	if !container.ImageExists(container.DefaultImage) {
		return fmt.Errorf("container image not found: %s\nRun: cd container && ./build.sh --load", container.DefaultImage)
	}

	// Ensure log directory exists
	if err := ensureLogDir(); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	if err := container.EnsureHistoryVolume(); err != nil {
		return fmt.Errorf("create history volume: %w", err)
	}

	// Update session status
	sess.Status = state.StatusRunning
	if err := state.Update(repoPath, sess); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	cmd.Println("Starting sandboxed execution...")
	cmd.Printf("  Session:   %s\n", sess.ID)
	cmd.Printf("  Worktree:  %s\n", sess.WorktreePath)
	cmd.Printf("  Container: %s\n", container.DefaultImage)
	cmd.Printf("  Log:       %s\n", sess.LogPath)
	cmd.Println()

	// Create log file
	logFile, err := os.Create(sess.LogPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		fmt.Print("\r\033[K")
		cmd.Println("Received interrupt, stopping...")
		cancel()
	}()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	// Run container
	done := make(chan error, 1)
	go func() {
		done <- container.Run(ctx, container.RunOptions{
			Image:        container.DefaultImage,
			WorktreePath: sess.WorktreePath,
			HomeDir:      home,
			SpecPath:     planPath,
			Interactive:  false,
			LogWriter:    logFile,
		})
	}()

	// Spinner with elapsed time
	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case runErr := <-done:
			fmt.Print("\r\033[K")
			elapsed := time.Since(start).Round(time.Second)

			// Determine final status from COMPLETION.md
			finalStatus := state.StatusFailed
			if content, err := os.ReadFile(completionPath); err == nil {
				if strings.Contains(string(content), "Status: SUCCESS") {
					finalStatus = state.StatusSuccess
				} else if strings.Contains(string(content), "Status: BLOCKED") {
					finalStatus = state.StatusBlocked
				}
			}

			sess.Status = finalStatus
			sess.CompletedAt = time.Now()
			if runErr != nil {
				sess.Error = runErr.Error()
			}
			state.Update(repoPath, sess)

			switch finalStatus {
			case state.StatusSuccess:
				cmd.Printf("✓ Completed in %s\n", elapsed)
				cmd.Println("Next step: claude-sandbox ship")
			case state.StatusBlocked:
				cmd.Printf("⚠ Blocked after %s\n", elapsed)
				cmd.Println("Check COMPLETION.md for details. Run 'claude-sandbox execute' to retry.")
			default:
				cmd.Printf("✗ Failed after %s\n", elapsed)
			}

			return runErr

		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			fmt.Printf("\r%s Claude working... %s", spinChars[i%len(spinChars)], elapsed)
			i++
		}
	}
}

func ensureLogDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	logDir := filepath.Join(home, ".claude", "sandbox-sessions")
	return os.MkdirAll(logDir, 0755)
}
```

- [ ] **Step 2: Delete run.go**

```bash
rm internal/cli/run.go
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./...`
Expected: Build errors (other commands not updated yet)

- [ ] **Step 4: Commit**

```bash
git add internal/cli/execute.go
git rm internal/cli/run.go
git commit -m "feat(cli): add execute command, remove run"
```

---

### Task 8: Update status command

**Files:**
- Modify: `internal/cli/status.go`

- [ ] **Step 1: Rewrite status.go**

```go
// internal/cli/status.go
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newStatusCommand() *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of sandbox session",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, sessionFlag)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name")

	return cmd
}

func runStatus(cmd *cobra.Command, sessionFlag string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	// Basic session info
	cmd.Printf("Session: %s\n", sess.ID)
	if sess.Name != "" {
		cmd.Printf("Name:    %s\n", sess.Name)
	}
	cmd.Printf("Branch:  %s\n", sess.Branch)
	cmd.Printf("Status:  %s\n", sess.Status)
	cmd.Printf("Path:    %s\n", sess.WorktreePath)

	elapsed := time.Since(sess.CreatedAt)
	if !sess.CompletedAt.IsZero() {
		elapsed = sess.CompletedAt.Sub(sess.CreatedAt)
	}
	cmd.Printf("Elapsed: %s\n", elapsed.Round(time.Second))
	cmd.Println()

	// Status-specific output
	switch sess.Status {
	case state.StatusSpeccing:
		cmd.Println("Claude session in progress for spec creation.")
	case state.StatusReady:
		cmd.Println("Spec complete. Run 'claude-sandbox execute' to start.")
	case state.StatusRunning:
		// AI-powered analysis
		logContent := readLogTail(sess.LogPath, 500)
		if logContent == "" {
			cmd.Println("Execution in progress. Log not available yet.")
			return nil
		}

		if !claudeAvailable() {
			cmd.Println("Execution in progress. (Claude CLI not available for analysis)")
			return nil
		}

		// Show spinner while analyzing
		spinChars := []string{"|", "/", "-", "\\"}
		done := make(chan string, 1)
		ctx := cmd.Context()
		go func() {
			done <- analyzeLog(ctx, logContent)
		}()

		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				fmt.Print("\r\033[K")
				return ctx.Err()
			case analysis := <-done:
				fmt.Print("\r\033[K")
				if analysis == "" {
					cmd.Println("Execution in progress.")
				} else {
					cmd.Println(analysis)
				}
				return nil
			case <-ticker.C:
				fmt.Printf("\r%s Analyzing...", spinChars[i%len(spinChars)])
				i++
			}
		}

	case state.StatusSuccess:
		cmd.Println("✓ Completed successfully. Run 'claude-sandbox ship' to create PR.")
	case state.StatusFailed:
		cmd.Printf("✗ Failed: %s\n", sess.Error)
		cmd.Println("Run 'claude-sandbox execute' to retry.")
	case state.StatusBlocked:
		cmd.Println("⚠ Blocked: quality gates could not be satisfied.")
		cmd.Println("Check COMPLETION.md for details. Run 'claude-sandbox execute' to retry.")
	}

	return nil
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./internal/cli/...`
Expected: May have errors if analyze.go/logutil.go imports changed

- [ ] **Step 3: Commit**

```bash
git add internal/cli/status.go
git commit -m "refactor(cli): update status command to use state resolution"
```

---

### Task 9: Update stop command

**Files:**
- Modify: `internal/cli/stop.go`

- [ ] **Step 1: Rewrite stop.go**

```go
// internal/cli/stop.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/container"
	"github.com/dakaneye/claude-sandbox/internal/state"
)

func newStopCommand() *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running sandbox session and container",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(cmd, sessionFlag)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name")

	return cmd
}

func runStop(cmd *cobra.Command, sessionFlag string) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	if sess.Status != state.StatusRunning {
		return fmt.Errorf("session is not running (status: %s)", sess.Status)
	}

	// Stop the Docker container if running
	if container.IsRunning(sess.WorktreePath) {
		cmd.Println("Stopping container...")
		if err := container.Stop(sess.WorktreePath); err != nil {
			cmd.PrintErrf("Warning: %v\n", err)
		}
	}

	sess.Status = state.StatusFailed
	sess.Error = "stopped by user"
	if err := state.Update(repoPath, sess); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	cmd.Println("Session stopped.")
	return nil
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./internal/cli/...`
Expected: Builds successfully

- [ ] **Step 3: Commit**

```bash
git add internal/cli/stop.go
git commit -m "refactor(cli): update stop command to use state resolution"
```

---

### Task 10: Update ship command

**Files:**
- Modify: `internal/cli/ship.go`

- [ ] **Step 1: Rewrite ship.go**

```go
// internal/cli/ship.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

func newShipCommand() *cobra.Command {
	var sessionFlag string
	var skipReview bool
	var keepWorktree bool

	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Create PR after reviewing completed work",
		Long: `Creates a PR for the completed sandbox work using the /create-pr skill.

Prerequisites:
  - COMPLETION.md must exist with SUCCESS status
  - User must review and confirm before PR creation

The command:
  1. Validates COMPLETION.md exists with SUCCESS status
  2. Prompts user to review COMPLETION.md
  3. Prompts for confirmation
  4. Invokes Claude with /create-pr skill
  5. Optionally cleans up the worktree`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShip(cmd, sessionFlag, skipReview, keepWorktree)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name")
	cmd.Flags().BoolVar(&skipReview, "skip-review", false, "Skip COMPLETION.md review prompt")
	cmd.Flags().BoolVar(&keepWorktree, "keep-worktree", false, "Don't clean up worktree after shipping")

	return cmd
}

func runShip(cmd *cobra.Command, sessionFlag string, skipReview, keepWorktree bool) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sess, err := state.ResolveSession(repoPath, sessionFlag)
	if err != nil {
		return err
	}

	completionPath := filepath.Join(sess.WorktreePath, "COMPLETION.md")
	if _, err := os.Stat(completionPath); os.IsNotExist(err) {
		return fmt.Errorf("COMPLETION.md not found. Run 'claude-sandbox execute' first")
	}

	content, err := os.ReadFile(completionPath)
	if err != nil {
		return fmt.Errorf("read COMPLETION.md: %w", err)
	}

	if !strings.Contains(string(content), "Status: SUCCESS") {
		return fmt.Errorf("COMPLETION.md does not show SUCCESS status. Cannot ship incomplete work")
	}

	if !skipReview {
		if !promptYesNo(cmd, "Review COMPLETION.md before shipping?", true) {
			return fmt.Errorf("shipping cancelled")
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "less"
		}
		editorCmd := exec.Command(editor, completionPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr
		_ = editorCmd.Run()
	}

	if !promptYesNo(cmd, "Ship this work?", false) {
		return fmt.Errorf("shipping cancelled")
	}

	cmd.Println("Launching Claude to create PR via /create-pr...")

	claudeCmd := exec.Command("claude", "--dangerously-skip-permissions", "/create-pr")
	claudeCmd.Dir = sess.WorktreePath
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr

	if err := claudeCmd.Run(); err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	if !keepWorktree {
		if promptYesNo(cmd, "Clean up worktree and session?", true) {
			if err := worktree.Remove(sess.WorktreePath); err != nil {
				cmd.PrintErrf("Warning: failed to remove worktree: %v\n", err)
			}
			if err := state.Remove(repoPath, sess.ID); err != nil {
				cmd.PrintErrf("Warning: failed to remove session: %v\n", err)
			} else {
				cmd.Println("Session cleaned up.")
			}
		}
	}

	return nil
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./internal/cli/...`
Expected: Builds successfully

- [ ] **Step 3: Commit**

```bash
git add internal/cli/ship.go
git commit -m "refactor(cli): update ship command to use state resolution"
```

---

### Task 11: Rewrite clean command

**Files:**
- Modify: `internal/cli/clean.go`

- [ ] **Step 1: Rewrite clean.go**

```go
// internal/cli/clean.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dakaneye/claude-sandbox/internal/state"
	"github.com/dakaneye/claude-sandbox/internal/worktree"
)

func newCleanCommand() *cobra.Command {
	var sessionFlag string
	var all bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove sandbox sessions and worktrees",
		Long: `Removes sandbox sessions and their associated worktrees.

Without flags, shows an interactive picker to select sessions to remove.
Use --session to remove a specific session, or --all to remove all sessions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClean(cmd, sessionFlag, all)
		},
	}

	cmd.Flags().StringVar(&sessionFlag, "session", "", "Session ID or name to remove")
	cmd.Flags().BoolVar(&all, "all", false, "Remove all sessions")

	return cmd
}

func runClean(cmd *cobra.Command, sessionFlag string, all bool) error {
	repoPath, err := findRepoRoot()
	if err != nil {
		return err
	}

	sessions, err := state.List(repoPath)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		cmd.Println("No sessions found.")
		return nil
	}

	var toRemove []*state.Session

	if sessionFlag != "" {
		// Remove specific session
		sess, err := state.Get(repoPath, sessionFlag)
		if err != nil {
			return err
		}
		toRemove = append(toRemove, sess)
	} else if all {
		// Remove all
		toRemove = sessions
	} else {
		// Interactive picker
		cmd.Printf("Found %d session(s):\n\n", len(sessions))
		for i, sess := range sessions {
			name := sess.ID
			if sess.Name != "" {
				name = fmt.Sprintf("%s (%s)", sess.Name, sess.ID)
			}
			cmd.Printf("  [%d] %s - %s\n", i+1, name, sess.Status)
		}
		cmd.Println()

		if !promptYesNo(cmd, "Remove all sessions?", false) {
			cmd.Println("Cancelled.")
			return nil
		}
		toRemove = sessions
	}

	if len(toRemove) > 1 && !all {
		if !promptYesNo(cmd, fmt.Sprintf("Remove %d sessions?", len(toRemove)), false) {
			return nil
		}
	}

	for _, sess := range toRemove {
		cmd.Printf("Removing %s...\n", sess.ID)

		// Remove worktree
		if err := worktree.Remove(sess.WorktreePath); err != nil {
			cmd.PrintErrf("  Warning (worktree): %v\n", err)
		}

		// Remove session state
		if err := state.Remove(repoPath, sess.ID); err != nil {
			cmd.PrintErrf("  Warning (session): %v\n", err)
		}
	}

	cmd.Println("Done.")
	return nil
}
```

- [ ] **Step 2: Run build to verify**

Run: `go build ./internal/cli/...`
Expected: Builds successfully

- [ ] **Step 3: Commit**

```bash
git add internal/cli/clean.go
git commit -m "refactor(cli): rewrite clean command to use state package"
```

---

### Task 12: Remove logs command and old session package

**Files:**
- Delete: `internal/cli/logs.go`
- Delete: `internal/session/session.go`

- [ ] **Step 1: Delete logs.go**

```bash
rm internal/cli/logs.go
```

- [ ] **Step 2: Delete session package**

```bash
rm -rf internal/session/
```

- [ ] **Step 3: Run build to verify**

Run: `go build ./...`
Expected: Builds successfully

- [ ] **Step 4: Run tests**

Run: `go test -v ./...`
Expected: Tests pass (some old tests may need removal)

- [ ] **Step 5: Commit**

```bash
git rm internal/cli/logs.go
git rm -r internal/session/
git commit -m "refactor: remove logs command and old session package"
```

---

### Task 13: Update and clean up tests

**Files:**
- Delete: `internal/cli/init_test.go`
- Delete: `internal/cli/logs_test.go`
- Modify: `internal/cli/root_test.go`
- Modify: Other test files as needed

- [ ] **Step 1: Remove obsolete test files**

```bash
rm internal/cli/init_test.go internal/cli/logs_test.go
rm internal/session/session_test.go 2>/dev/null || true
```

- [ ] **Step 2: Update root_test.go**

```go
// internal/cli/root_test.go
package cli

import (
	"testing"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand("test-version")

	if cmd.Use != "claude-sandbox" {
		t.Errorf("expected Use 'claude-sandbox', got %q", cmd.Use)
	}

	// Check subcommands exist
	expectedCmds := []string{"spec", "execute", "status", "stop", "ship", "clean"}
	for _, name := range expectedCmds {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Use == name || sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found", name)
		}
	}

	// Verify removed commands don't exist
	removedCmds := []string{"init", "run", "logs"}
	for _, name := range removedCmds {
		for _, sub := range cmd.Commands() {
			if sub.Use == name || sub.Name() == name {
				t.Errorf("removed command %q still exists", name)
			}
		}
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `go test -v ./...`
Expected: All tests pass

- [ ] **Step 4: Run linter**

Run: `golangci-lint run ./...`
Expected: No issues

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test: update tests for new CLI structure"
```

---

### Task 14: Final verification and cleanup

**Files:**
- None (verification only)

- [ ] **Step 1: Build the binary**

Run: `go build -o bin/claude-sandbox ./cmd/claude-sandbox`
Expected: Builds successfully

- [ ] **Step 2: Verify help output**

Run: `./bin/claude-sandbox --help`
Expected: Shows spec, execute, status, stop, ship, clean commands

- [ ] **Step 3: Run all tests**

Run: `go test -race ./...`
Expected: All tests pass

- [ ] **Step 4: Run linter**

Run: `golangci-lint run ./...`
Expected: No issues

- [ ] **Step 5: Tidy modules**

Run: `go mod tidy`
Expected: No changes needed

- [ ] **Step 6: Final commit if needed**

```bash
git status
# If any changes, commit them
```

- [ ] **Step 7: Run code review**

Run: `/review-code`
Expected: Grade A
