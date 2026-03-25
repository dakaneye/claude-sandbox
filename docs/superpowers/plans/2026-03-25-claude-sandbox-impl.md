# Claude Sandbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a CLI tool that enables autonomous Claude Code execution in a sandboxed container with quality gates.

**Architecture:** Go CLI using cobra/viper orchestrates Docker containers running Claude Code. Git worktrees isolate changes. Bash hooks block external side effects. COMPLETION.md serves as the handoff artifact.

**Tech Stack:** Go 1.22+, cobra, viper, Docker API, git CLI, apko for container images

**Spec:** `docs/specs/2026-03-25-claude-sandbox-design.md`

---

## File Structure

```
claude-sandbox/
├── cmd/
│   └── claude-sandbox/
│       └── main.go                 # CLI entry point
├── internal/
│   ├── cli/
│   │   ├── root.go                 # Root command, global flags, version
│   │   ├── init.go                 # init subcommand
│   │   ├── run.go                  # run subcommand
│   │   ├── ship.go                 # ship subcommand
│   │   ├── status.go               # status subcommand
│   │   ├── logs.go                 # logs subcommand
│   │   ├── stop.go                 # stop subcommand
│   │   └── clean.go                # clean subcommand
│   ├── worktree/
│   │   ├── worktree.go             # Git worktree operations
│   │   └── worktree_test.go
│   ├── container/
│   │   ├── container.go            # Docker container operations
│   │   ├── mounts.go               # Volume mount configuration
│   │   └── container_test.go
│   ├── config/
│   │   ├── config.go               # .claude-sandbox.yaml parsing
│   │   └── config_test.go
│   └── session/
│       ├── session.go              # Session state (running, completed)
│       └── session_test.go
├── container/
│   ├── claude-sandbox.apko.yaml    # apko image config
│   ├── build.sh                    # Image build script
│   └── Dockerfile.prebake          # Pre-bake Claude Code layer
├── hooks/
│   └── sandbox-guard.sh            # External action gate hook
├── go.mod
├── go.sum
├── Makefile                        # Build, test, install targets
└── README.md
```

---

## Phase 1: Project Bootstrap

### Task 1.1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `Makefile`

- [ ] **Step 1: Initialize go module**

```bash
cd ~/dev/personal/claude-sandbox
go mod init github.com/samueldacanay/claude-sandbox
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
```

- [ ] **Step 3: Create Makefile**

```makefile
.PHONY: build test lint install clean

BINARY := claude-sandbox
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/claude-sandbox

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

install: build
	cp bin/$(BINARY) ~/go/bin/

clean:
	rm -rf bin/
```

- [ ] **Step 4: Verify setup**

```bash
go mod tidy
```

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum Makefile
git commit -m "chore: initialize go module with cobra/viper deps"
```

---

### Task 1.2: Create CLI Entry Point

**Files:**
- Create: `cmd/claude-sandbox/main.go`
- Create: `internal/cli/root.go`

- [ ] **Step 1: Write test for root command**

Create `internal/cli/root_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestRootCommand_Version(t *testing.T) {
	cmd := NewRootCommand("test-version")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("test-version")) {
		t.Errorf("expected version in output, got: %s", output)
	}
}

func TestRootCommand_Help(t *testing.T) {
	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	_ = cmd.Execute()

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("claude-sandbox")) {
		t.Errorf("expected 'claude-sandbox' in help output, got: %s", output)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Create root command**

Create `internal/cli/root.go`:

```go
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

	return cmd
}
```

- [ ] **Step 4: Create main.go**

Create `cmd/claude-sandbox/main.go`:

```go
package main

import (
	"os"

	"github.com/samueldacanay/claude-sandbox/internal/cli"
)

var version = "dev"

func main() {
	cmd := cli.NewRootCommand(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/cli/... -v
```

Expected: PASS

- [ ] **Step 6: Verify build**

```bash
make build
./bin/claude-sandbox --version
./bin/claude-sandbox --help
```

- [ ] **Step 7: Commit**

```bash
git add cmd/ internal/cli/
git commit -m "feat: add CLI entry point with root command"
```

---

## Phase 2: Container Image

### Task 2.1: Create apko Configuration

**Files:**
- Create: `container/claude-sandbox.apko.yaml`

- [ ] **Step 1: Create apko config**

Create `container/claude-sandbox.apko.yaml`:

```yaml
# Claude Sandbox - Fat Base Image
contents:
  repositories:
    - https://packages.wolfi.dev/os
  keyring:
    - https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
  packages:
    # Base system
    - wolfi-baselayout
    - ca-certificates-bundle
    - bash
    - coreutils
    - findutils

    # Node.js runtime (Claude Code)
    - nodejs-22
    - npm

    # Go development
    - go
    - golangci-lint

    # Python development
    - python-3.12
    - py3-pip

    # Cloud/infra tools
    - kubectl
    - terraform
    - chainctl
    - gh

    # Dev utilities
    - git
    - make
    - shellcheck
    - curl
    - wget
    - jq
    - yq
    - ripgrep
    - fd
    - fzf
    - tree
    - grep
    - sed
    - gawk

accounts:
  users:
    - username: claude
      uid: 65532
      gid: 65532
      shell: /bin/bash
  groups:
    - groupname: claude
      gid: 65532
  run-as: claude

work-dir: /workspace

entrypoint:
  command: /bin/bash

environment:
  PATH: /home/claude/.npm-global/bin:/home/claude/go/bin:/usr/local/bin:/usr/bin:/bin
  NPM_CONFIG_PREFIX: /home/claude/.npm-global
  GOPATH: /home/claude/go
  NODE_ENV: production
  TERM: xterm-256color

paths:
  - path: /workspace
    type: directory
    uid: 65532
    gid: 65532
    permissions: 0o755
  - path: /home/claude
    type: directory
    uid: 65532
    gid: 65532
    permissions: 0o755
  - path: /home/claude/.npm-global
    type: directory
    uid: 65532
    gid: 65532
    permissions: 0o755
  - path: /home/claude/go
    type: directory
    uid: 65532
    gid: 65532
    permissions: 0o755
  - path: /home/claude/.claude
    type: directory
    uid: 65532
    gid: 65532
    permissions: 0o755

archs:
  - amd64
  - arm64

annotations:
  org.opencontainers.image.title: claude-sandbox
  org.opencontainers.image.description: Sandboxed environment for autonomous Claude Code execution
  org.opencontainers.image.source: https://github.com/samueldacanay/claude-sandbox
```

- [ ] **Step 2: Commit**

```bash
git add container/claude-sandbox.apko.yaml
git commit -m "feat: add apko config for fat base image"
```

---

### Task 2.2: Create Build Script

**Files:**
- Create: `container/build.sh`
- Create: `container/Dockerfile.prebake`

- [ ] **Step 1: Create build script**

Create `container/build.sh`:

```bash
#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="${IMAGE_NAME:-claude-sandbox}"
TAG="${TAG:-latest}"

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Build the claude-sandbox container image.

Options:
    --load          Load image into Docker after build
    --push REGISTRY Push to registry (e.g., ghcr.io/user)
    --arch ARCH     Build for specific arch (amd64, arm64)
    --no-prebake    Skip pre-baking Claude Code into image
    -h, --help      Show this help

Examples:
    $(basename "$0") --load
    $(basename "$0") --push ghcr.io/samueldacanay
EOF
}

main() {
    local load=false
    local push=""
    local arch=""
    local prebake=true

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --load) load=true; shift ;;
            --push) push="$2"; shift 2 ;;
            --arch) arch="$2"; shift 2 ;;
            --no-prebake) prebake=false; shift ;;
            -h|--help) usage; exit 0 ;;
            *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
        esac
    done

    echo "Building claude-sandbox base image..."

    local apko_args=("build" "${SCRIPT_DIR}/claude-sandbox.apko.yaml" "${IMAGE_NAME}:base" "${SCRIPT_DIR}/${IMAGE_NAME}-base.tar")

    if [[ -n "$arch" ]]; then
        apko_args+=("--arch" "$arch")
    fi

    apko "${apko_args[@]}"

    echo "Loading base image into Docker..."
    docker load < "${SCRIPT_DIR}/${IMAGE_NAME}-base.tar"

    if [[ "$prebake" == true ]]; then
        echo "Pre-baking Claude Code into image..."
        docker build -t "${IMAGE_NAME}:${TAG}" -f "${SCRIPT_DIR}/Dockerfile.prebake" "${SCRIPT_DIR}"
    else
        docker tag "${IMAGE_NAME}:base" "${IMAGE_NAME}:${TAG}"
    fi

    if [[ "$load" == true ]]; then
        echo "Image ready: ${IMAGE_NAME}:${TAG}"
    fi

    if [[ -n "$push" ]]; then
        local full_tag="${push}/${IMAGE_NAME}:${TAG}"
        echo "Pushing to ${full_tag}..."
        docker tag "${IMAGE_NAME}:${TAG}" "${full_tag}"
        docker push "${full_tag}"
    fi

    # Cleanup
    rm -f "${SCRIPT_DIR}/${IMAGE_NAME}-base.tar"

    echo "Done."
}

main "$@"
```

- [ ] **Step 2: Create Dockerfile for pre-baking**

Create `container/Dockerfile.prebake`:

```dockerfile
FROM claude-sandbox:base

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Verify installation
RUN claude --version || echo "Claude Code installed"
```

- [ ] **Step 3: Make build script executable**

```bash
chmod +x container/build.sh
```

- [ ] **Step 4: Commit**

```bash
git add container/build.sh container/Dockerfile.prebake
git commit -m "feat: add container build script with prebake support"
```

---

## Phase 3: Hooks

### Task 3.1: Create sandbox-guard Hook

**Files:**
- Create: `hooks/sandbox-guard.sh`

- [ ] **Step 1: Create hook script**

Create `hooks/sandbox-guard.sh`:

```bash
#!/usr/bin/env bash
# sandbox-guard.sh - Block external side effects in sandbox mode
# Exit 0 = allow, Exit 2 = block
set -euo pipefail

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
TOOL_INPUT=$(echo "$INPUT" | jq -r '.tool_input // empty')

# ============================================
# MCP Tool Gates
# ============================================
if [[ "$TOOL_NAME" == mcp__* ]]; then
    # Linear MCP - block writes
    BLOCKED_MCP=(
        "mcp__linear__createIssue"
        "mcp__linear__updateIssue"
        "mcp__linear__createComment"
        "mcp__linear__addLabel"
        "mcp__linear__removeLabel"
        "mcp__linear__archiveIssue"
    )

    for blocked in "${BLOCKED_MCP[@]}"; do
        if [[ "$TOOL_NAME" == "$blocked" ]]; then
            echo "BLOCKED: MCP write operation not allowed in sandbox" >&2
            echo "  Tool: $TOOL_NAME" >&2
            echo "  Use 'claude-sandbox ship' after review to perform external actions." >&2
            exit 2
        fi
    done

    # Allow all other MCP (reads)
    exit 0
fi

# ============================================
# Bash Command Gates
# ============================================
if [[ "$TOOL_NAME" == "Bash" ]]; then
    CMD=$(echo "$TOOL_INPUT" | jq -r '.command // empty')

    # --- Always blocked (catastrophic) ---
    CATASTROPHIC=(
        'rm -rf /'
        'rm -rf ~'
        'rm -rf \$HOME'
        'rm -rf /Users'
        'rm -rf /home'
        'chmod -R 777 /'
        'chown -R.*/'
    )

    for pattern in "${CATASTROPHIC[@]}"; do
        if echo "$CMD" | grep -qiE "$pattern"; then
            echo "BLOCKED: Catastrophic command" >&2
            exit 2
        fi
    done

    # --- Git push (any form) ---
    if echo "$CMD" | grep -qE '\bgit\s+push\b'; then
        echo "BLOCKED: git push not allowed in sandbox" >&2
        echo "  Use 'claude-sandbox ship' after review." >&2
        exit 2
    fi

    # --- GitHub CLI writes ---
    GH_WRITE_PATTERNS=(
        'gh\s+pr\s+create'
        'gh\s+pr\s+merge'
        'gh\s+pr\s+close'
        'gh\s+pr\s+comment'
        'gh\s+pr\s+review'
        'gh\s+issue\s+create'
        'gh\s+issue\s+close'
        'gh\s+issue\s+comment'
        'gh\s+release\s+create'
    )

    for pattern in "${GH_WRITE_PATTERNS[@]}"; do
        if echo "$CMD" | grep -qE "$pattern"; then
            echo "BLOCKED: gh write operation not allowed in sandbox" >&2
            echo "  Command: $CMD" >&2
            echo "  Use 'claude-sandbox ship' after review." >&2
            exit 2
        fi
    done

    # --- HTTP write methods ---
    HTTP_WRITE_PATTERNS=(
        'curl.*-X\s*POST'
        'curl.*-X\s*PUT'
        'curl.*-X\s*PATCH'
        'curl.*-X\s*DELETE'
        'curl.*--data'
        'curl.*-d\s'
        'curl.*--json'
        'wget.*--post-data'
        'wget.*--post-file'
    )

    for pattern in "${HTTP_WRITE_PATTERNS[@]}"; do
        if echo "$CMD" | grep -qiE "$pattern"; then
            echo "BLOCKED: HTTP write operation not allowed in sandbox" >&2
            echo "  Command: $CMD" >&2
            exit 2
        fi
    done

    # Allow everything else
    exit 0
fi

# ============================================
# All other tools - allow
# ============================================
exit 0
```

- [ ] **Step 2: Make executable**

```bash
chmod +x hooks/sandbox-guard.sh
```

- [ ] **Step 3: Test hook locally**

```bash
# Test: should allow git commit
echo '{"tool_name": "Bash", "tool_input": {"command": "git commit -m test"}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 0

# Test: should block git push
echo '{"tool_name": "Bash", "tool_input": {"command": "git push origin main"}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 2

# Test: should block gh pr create
echo '{"tool_name": "Bash", "tool_input": {"command": "gh pr create --title test"}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 2

# Test: should allow curl GET
echo '{"tool_name": "Bash", "tool_input": {"command": "curl https://api.github.com"}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 0

# Test: should block curl POST
echo '{"tool_name": "Bash", "tool_input": {"command": "curl -X POST https://api.example.com"}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 2

# Test: should block Linear MCP write
echo '{"tool_name": "mcp__linear__createIssue", "tool_input": {}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 2

# Test: should allow Linear MCP read
echo '{"tool_name": "mcp__linear__getIssue", "tool_input": {}}' | ./hooks/sandbox-guard.sh
echo "Exit code: $?"  # Should be 0
```

- [ ] **Step 4: Commit**

```bash
git add hooks/sandbox-guard.sh
git commit -m "feat: add sandbox-guard hook for external action blocking"
```

---

## Phase 4: Worktree Module

### Task 4.1: Implement Worktree Operations

**Files:**
- Create: `internal/worktree/worktree.go`
- Create: `internal/worktree/worktree_test.go`

- [ ] **Step 1: Write tests**

Create `internal/worktree/worktree_test.go`:

```go
package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"touch", "README.md"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup command %v failed: %v", args, err)
		}
	}

	return dir
}

func TestIsGitRepo(t *testing.T) {
	t.Run("valid repo", func(t *testing.T) {
		repo := setupTestRepo(t)
		if !IsGitRepo(repo) {
			t.Error("expected true for valid git repo")
		}
	})

	t.Run("not a repo", func(t *testing.T) {
		dir := t.TempDir()
		if IsGitRepo(dir) {
			t.Error("expected false for non-git directory")
		}
	})
}

func TestCreate(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := Create(repo)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = Remove(wt.Path) }()

	// Verify worktree exists
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}

	// Verify branch name format
	if !strings.HasPrefix(wt.Branch, "sandbox/") {
		t.Errorf("expected branch prefix 'sandbox/', got: %s", wt.Branch)
	}

	// Verify it's a git worktree
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = wt.Path
	if err := cmd.Run(); err != nil {
		t.Error("worktree is not a valid git working tree")
	}
}

func TestList(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a worktree
	wt, err := Create(repo)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = Remove(wt.Path) }()

	// List worktrees
	worktrees, err := List(repo)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have at least 2 (main + sandbox)
	if len(worktrees) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(worktrees))
	}

	// Find our sandbox worktree
	found := false
	for _, w := range worktrees {
		if strings.HasPrefix(w.Branch, "sandbox/") {
			found = true
			break
		}
	}
	if !found {
		t.Error("sandbox worktree not found in list")
	}
}

func TestRemove(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := Create(repo)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	path := wt.Path

	err = Remove(path)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after removal")
	}
}

func TestDetect(t *testing.T) {
	repo := setupTestRepo(t)

	wt, err := Create(repo)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	defer func() { _ = Remove(wt.Path) }()

	t.Run("from worktree root", func(t *testing.T) {
		detected, err := Detect(wt.Path)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if detected.Path != wt.Path {
			t.Errorf("expected path %s, got %s", wt.Path, detected.Path)
		}
	})

	t.Run("from subdirectory", func(t *testing.T) {
		subdir := filepath.Join(wt.Path, "subdir")
		if err := os.Mkdir(subdir, 0755); err != nil {
			t.Fatal(err)
		}

		detected, err := Detect(subdir)
		if err != nil {
			t.Fatalf("Detect failed: %v", err)
		}
		if detected.Path != wt.Path {
			t.Errorf("expected path %s, got %s", wt.Path, detected.Path)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/worktree/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement worktree module**

Create `internal/worktree/worktree.go`:

```go
package worktree

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Worktree represents a git worktree
type Worktree struct {
	Path   string
	Branch string
	Repo   string
}

// IsGitRepo checks if the path is inside a git repository
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

// Create creates a new git worktree for sandbox work
func Create(repoPath string) (*Worktree, error) {
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	if !IsGitRepo(absRepo) {
		return nil, fmt.Errorf("not a git repository: %s", absRepo)
	}

	// Generate branch and path names
	hash := randomHash(6)
	date := time.Now().Format("2006-01-02")
	branch := fmt.Sprintf("sandbox/%s-%s", date, hash)
	worktreePath := fmt.Sprintf("%s-sandbox-%s", absRepo, hash)

	// Create the worktree
	cmd := exec.Command("git", "worktree", "add", "-b", branch, worktreePath)
	cmd.Dir = absRepo
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("create worktree: %w\noutput: %s", err, output)
	}

	return &Worktree{
		Path:   worktreePath,
		Branch: branch,
		Repo:   absRepo,
	}, nil
}

// List returns all worktrees for a repository
func List(repoPath string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	var worktrees []Worktree
	var current Worktree

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
			current.Repo = repoPath
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			// Convert refs/heads/branch to branch
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		}
	}

	// Don't forget the last one
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// Remove removes a worktree and its branch
func Remove(worktreePath string) error {
	// Find the main repo to run git commands
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		// Worktree may already be partially removed, try direct removal
		return os.RemoveAll(worktreePath)
	}

	gitDir := strings.TrimSpace(string(output))
	repoPath := filepath.Dir(gitDir)

	// Get branch name before removal
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = worktreePath
	branchOutput, _ := cmd.Output()
	branch := strings.TrimSpace(string(branchOutput))

	// Remove the worktree
	cmd = exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		// Try manual removal
		if rmErr := os.RemoveAll(worktreePath); rmErr != nil {
			return fmt.Errorf("remove worktree: %w", err)
		}
		// Prune worktree references
		pruneCmd := exec.Command("git", "worktree", "prune")
		pruneCmd.Dir = repoPath
		_ = pruneCmd.Run()
	}

	// Delete the branch if it's a sandbox branch
	if strings.HasPrefix(branch, "sandbox/") {
		cmd = exec.Command("git", "branch", "-D", branch)
		cmd.Dir = repoPath
		_ = cmd.Run() // Best effort, branch may not exist
	}

	return nil
}

// Detect finds the worktree containing the given path
func Detect(path string) (*Worktree, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Get the toplevel of the worktree
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not inside a git worktree: %w", err)
	}
	toplevel := strings.TrimSpace(string(output))

	// Get the branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = toplevel
	branchOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Get the main repo path
	cmd = exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = toplevel
	repoOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get repo path: %w", err)
	}
	repo := filepath.Dir(strings.TrimSpace(string(repoOutput)))

	return &Worktree{
		Path:   toplevel,
		Branch: branch,
		Repo:   repo,
	}, nil
}

// IsSandbox returns true if the worktree is a sandbox worktree
func (w *Worktree) IsSandbox() bool {
	return strings.HasPrefix(w.Branch, "sandbox/")
}

func randomHash(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)[:n]
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/worktree/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat: add worktree module for git worktree operations"
```

---

## Phase 5: Config Module

### Task 5.1: Implement Config Parsing

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check defaults
	if cfg.Gates.Build != "" {
		t.Error("expected empty default for build gate")
	}
	if cfg.Retries != 3 {
		t.Errorf("expected default retries=3, got %d", cfg.Retries)
	}
	if cfg.Timeout != "2h" {
		t.Errorf("expected default timeout=2h, got %s", cfg.Timeout)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()

	configContent := `
gates:
  build: "make build"
  lint: "make lint"
  test: "make test-all"
  security: "make security"
retries: 5
timeout: "4h"
`
	configPath := filepath.Join(dir, ".claude-sandbox.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Gates.Build != "make build" {
		t.Errorf("expected 'make build', got %s", cfg.Gates.Build)
	}
	if cfg.Gates.Test != "make test-all" {
		t.Errorf("expected 'make test-all', got %s", cfg.Gates.Test)
	}
	if cfg.Retries != 5 {
		t.Errorf("expected retries=5, got %d", cfg.Retries)
	}
	if cfg.Timeout != "4h" {
		t.Errorf("expected timeout=4h, got %s", cfg.Timeout)
	}
}

func TestDetectGateCommand(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		gate     string
		expected string
	}{
		{
			name:     "go build",
			files:    map[string]string{"go.mod": "module test"},
			gate:     "build",
			expected: "go build ./...",
		},
		{
			name:     "npm build",
			files:    map[string]string{"package.json": `{"scripts":{"build":"tsc"}}`},
			gate:     "build",
			expected: "npm run build",
		},
		{
			name:     "makefile",
			files:    map[string]string{"Makefile": "build:\n\techo build"},
			gate:     "build",
			expected: "make build",
		},
		{
			name:     "go test",
			files:    map[string]string{"go.mod": "module test", "foo_test.go": ""},
			gate:     "test",
			expected: "go test ./...",
		},
		{
			name:     "golangci-lint",
			files:    map[string]string{"go.mod": "module test"},
			gate:     "lint",
			expected: "golangci-lint run ./...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			cmd := DetectGateCommand(dir, tt.gate)
			if cmd != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, cmd)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement config module**

Create `internal/config/config.go`:

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents .claude-sandbox.yaml configuration
type Config struct {
	Gates   GatesConfig `mapstructure:"gates"`
	Retries int         `mapstructure:"retries"`
	Timeout string      `mapstructure:"timeout"`
}

// GatesConfig holds custom gate commands
type GatesConfig struct {
	Build    string `mapstructure:"build"`
	Lint     string `mapstructure:"lint"`
	Test     string `mapstructure:"test"`
	Security string `mapstructure:"security"`
}

// Load loads configuration from .claude-sandbox.yaml in the given directory
func Load(dir string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("retries", 3)
	v.SetDefault("timeout", "2h")

	// Look for config file
	v.SetConfigName(".claude-sandbox")
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)

	// Read config (ignore if not found)
	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// DetectGateCommand auto-detects the command for a gate based on project files
func DetectGateCommand(dir string, gate string) string {
	switch gate {
	case "build":
		return detectBuildCommand(dir)
	case "lint":
		return detectLintCommand(dir)
	case "test":
		return detectTestCommand(dir)
	case "security":
		return detectSecurityCommand(dir)
	default:
		return ""
	}
}

func detectBuildCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "build") {
		return "make build"
	}
	if fileExists(dir, "go.mod") {
		return "go build ./..."
	}
	if fileExists(dir, "package.json") && npmScriptExists(dir, "build") {
		return "npm run build"
	}
	return ""
}

func detectLintCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "lint") {
		return "make lint"
	}
	if fileExists(dir, "go.mod") {
		return "golangci-lint run ./..."
	}
	if fileExists(dir, "package.json") && npmScriptExists(dir, "lint") {
		return "npm run lint"
	}
	return ""
}

func detectTestCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "test") {
		return "make test"
	}
	if fileExists(dir, "go.mod") {
		return "go test ./..."
	}
	if fileExists(dir, "package.json") && npmScriptExists(dir, "test") {
		return "npm test"
	}
	return ""
}

func detectSecurityCommand(dir string) string {
	if fileExists(dir, "Makefile") && makeTargetExists(dir, "security") {
		return "make security"
	}
	if fileExists(dir, "go.mod") {
		return "govulncheck ./..."
	}
	if fileExists(dir, "package.json") {
		return "npm audit"
	}
	return ""
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func makeTargetExists(dir, target string) bool {
	// Simple check - look for target: in Makefile
	content, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		return false
	}
	// Very basic - just check if "target:" appears
	return filepath.Join(target+":") != "" && len(content) > 0 // Placeholder - always true if Makefile exists
}

func npmScriptExists(dir, script string) bool {
	content, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return false
	}

	_, exists := pkg.Scripts[script]
	return exists
}

// GetGateCommand returns the command for a gate, using config override or auto-detection
func (c *Config) GetGateCommand(dir string, gate string) string {
	// Check config override first
	switch gate {
	case "build":
		if c.Gates.Build != "" {
			return c.Gates.Build
		}
	case "lint":
		if c.Gates.Lint != "" {
			return c.Gates.Lint
		}
	case "test":
		if c.Gates.Test != "" {
			return c.Gates.Test
		}
	case "security":
		if c.Gates.Security != "" {
			return c.Gates.Security
		}
	}

	// Fall back to auto-detection
	return DetectGateCommand(dir, gate)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config module for .claude-sandbox.yaml parsing"
```

---

## Phase 6: Container Module

### Task 6.1: Implement Container Operations

**Files:**
- Create: `internal/container/container.go`
- Create: `internal/container/mounts.go`
- Create: `internal/container/container_test.go`

- [ ] **Step 1: Write tests**

Create `internal/container/container_test.go`:

```go
package container

import (
	"strings"
	"testing"
)

func TestBuildMounts(t *testing.T) {
	opts := MountOptions{
		WorktreePath: "/tmp/worktree",
		HomeDir:      "/Users/test",
	}

	mounts := BuildMounts(opts)

	// Check for required mounts
	requiredSources := []string{
		"/Users/test/.claude/settings.json",
		"/Users/test/.claude/hooks",
		"/Users/test/.gitconfig",
		"/Users/test/.ssh",
		"/tmp/worktree",
	}

	for _, src := range requiredSources {
		found := false
		for _, m := range mounts {
			if m.Source == src {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing mount for %s", src)
		}
	}

	// Check that worktree is read-write
	for _, m := range mounts {
		if m.Source == "/tmp/worktree" {
			if m.ReadOnly {
				t.Error("worktree mount should be read-write")
			}
			if m.Target != "/workspace" {
				t.Errorf("worktree should mount to /workspace, got %s", m.Target)
			}
		}
	}

	// Check that config mounts are read-only
	for _, m := range mounts {
		if strings.Contains(m.Source, ".claude/settings.json") && !m.ReadOnly {
			t.Error("settings.json should be read-only")
		}
	}
}

func TestBuildRunArgs(t *testing.T) {
	opts := RunOptions{
		Image:        "claude-sandbox:latest",
		WorktreePath: "/tmp/worktree",
		HomeDir:      "/Users/test",
		APIKey:       "sk-test",
		SpecPath:     "/tmp/worktree/spec.md",
	}

	args := BuildRunArgs(opts)

	// Should start with "run"
	if args[0] != "run" {
		t.Errorf("expected first arg 'run', got %s", args[0])
	}

	// Should have --rm
	found := false
	for _, arg := range args {
		if arg == "--rm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing --rm flag")
	}

	// Should have -it
	foundI, foundT := false, false
	for i, arg := range args {
		if arg == "-i" {
			foundI = true
		}
		if arg == "-t" {
			foundT = true
		}
		if arg == "-it" {
			foundI, foundT = true, true
		}
		_ = i
	}
	if !foundI || !foundT {
		t.Error("missing -it flags for interactive mode")
	}

	// Should set ANTHROPIC_API_KEY
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) && strings.HasPrefix(args[i+1], "ANTHROPIC_API_KEY=") {
			return // Found it
		}
	}
	t.Error("missing ANTHROPIC_API_KEY environment variable")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/container/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement mounts module**

Create `internal/container/mounts.go`:

```go
package container

import (
	"path/filepath"
)

// Mount represents a Docker volume mount
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// MountOptions configures mount generation
type MountOptions struct {
	WorktreePath string
	HomeDir      string
}

// BuildMounts generates the volume mounts for the sandbox container
func BuildMounts(opts MountOptions) []Mount {
	home := opts.HomeDir

	mounts := []Mount{
		// Read-only config mounts
		{
			Source:   filepath.Join(home, ".claude", "settings.json"),
			Target:   "/home/claude/.claude/settings.json",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".claude", "hooks"),
			Target:   "/home/claude/.claude/hooks",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".claude", "commands"),
			Target:   "/home/claude/.claude/commands",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".claude", "skills"),
			Target:   "/home/claude/.claude/skills",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".gitconfig"),
			Target:   "/home/claude/.gitconfig",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".ssh"),
			Target:   "/home/claude/.ssh",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".config", "gh"),
			Target:   "/home/claude/.config/gh",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".config", "chainctl"),
			Target:   "/home/claude/.config/chainctl",
			ReadOnly: true,
		},
		// Read-write mounts
		{
			Source:   opts.WorktreePath,
			Target:   "/workspace",
			ReadOnly: false,
		},
	}

	return mounts
}

// ToDockerArgs converts mounts to Docker -v arguments
func (m Mount) ToDockerArgs() []string {
	mountSpec := m.Source + ":" + m.Target
	if m.ReadOnly {
		mountSpec += ":ro"
	}
	return []string{"-v", mountSpec}
}
```

- [ ] **Step 4: Implement container module**

Create `internal/container/container.go`:

```go
package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	DefaultImage      = "claude-sandbox:latest"
	HistoryVolumeName = "claude-sandbox-history"
)

// RunOptions configures container execution
type RunOptions struct {
	Image        string
	WorktreePath string
	HomeDir      string
	APIKey       string
	SpecPath     string
	Timeout      string
	Interactive  bool
}

// BuildRunArgs generates docker run arguments
func BuildRunArgs(opts RunOptions) []string {
	args := []string{"run", "--rm"}

	if opts.Interactive {
		args = append(args, "-it")
	}

	// Add mounts
	mounts := BuildMounts(MountOptions{
		WorktreePath: opts.WorktreePath,
		HomeDir:      opts.HomeDir,
	})

	for _, m := range mounts {
		args = append(args, m.ToDockerArgs()...)
	}

	// Add history volume
	args = append(args, "-v", HistoryVolumeName+":/home/claude/.claude/history")

	// Environment variables
	args = append(args, "-e", "ANTHROPIC_API_KEY="+opts.APIKey)
	args = append(args, "-e", "HOME=/home/claude")

	// Working directory
	args = append(args, "--workdir", "/workspace")

	// Image
	args = append(args, opts.Image)

	return args
}

// Run executes Claude in a sandbox container
func Run(ctx context.Context, opts RunOptions) error {
	if opts.Image == "" {
		opts.Image = DefaultImage
	}

	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		if opts.APIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
	}

	if opts.HomeDir == "" {
		var err error
		opts.HomeDir, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
	}

	args := BuildRunArgs(opts)

	// Add the command to run Claude
	claudeCmd := buildClaudeCommand(opts.SpecPath)
	args = append(args, "/bin/bash", "-c", claudeCmd)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildClaudeCommand(specPath string) string {
	// If spec is a directory, read all markdown files
	// If spec is a file, use it directly
	return fmt.Sprintf(`claude --dangerously-skip-permissions "Implement the spec at %s. Follow quality gates: build, lint, test, security, spec coverage, commit hygiene, and /review-code with grade A. Write COMPLETION.md when done."`, specPath)
}

// ImageExists checks if the sandbox image exists locally
func ImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

// EnsureHistoryVolume creates the history volume if it doesn't exist
func EnsureHistoryVolume() error {
	cmd := exec.Command("docker", "volume", "inspect", HistoryVolumeName)
	if cmd.Run() == nil {
		return nil // Already exists
	}

	cmd = exec.Command("docker", "volume", "create", HistoryVolumeName)
	return cmd.Run()
}

// GetSessionLogPath returns the path for session logs
func GetSessionLogPath(sessionID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "sandbox-sessions", sessionID+".log")
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/container/... -v
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/container/
git commit -m "feat: add container module for Docker operations"
```

---

## Phase 7: Session Module

### Task 7.1: Implement Session State

**Files:**
- Create: `internal/session/session.go`
- Create: `internal/session/session_test.go`

- [ ] **Step 1: Write tests**

Create `internal/session/session_test.go`:

```go
package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSession_SaveLoad(t *testing.T) {
	dir := t.TempDir()

	s := &Session{
		ID:           "test-123",
		WorktreePath: "/tmp/worktree",
		SpecPath:     "/tmp/worktree/spec.md",
		Status:       StatusRunning,
		StartedAt:    time.Now(),
		ContainerID:  "abc123",
	}

	// Save
	if err := s.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	sessionFile := filepath.Join(dir, "session.json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("session file not created")
	}

	// Load
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != s.ID {
		t.Errorf("expected ID %s, got %s", s.ID, loaded.ID)
	}
	if loaded.Status != StatusRunning {
		t.Errorf("expected status Running, got %s", loaded.Status)
	}
}

func TestSession_Complete(t *testing.T) {
	s := &Session{
		ID:        "test-123",
		Status:    StatusRunning,
		StartedAt: time.Now().Add(-10 * time.Minute),
	}

	s.Complete(StatusSuccess)

	if s.Status != StatusSuccess {
		t.Errorf("expected status Success, got %s", s.Status)
	}
	if s.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
	if s.Duration() < 10*time.Minute {
		t.Errorf("expected duration >= 10m, got %v", s.Duration())
	}
}

func TestFindActive(t *testing.T) {
	dir := t.TempDir()

	// No session file
	_, err := FindActive(dir)
	if err == nil {
		t.Error("expected error for missing session")
	}

	// Create running session
	s := &Session{
		ID:        "test-123",
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}
	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}

	found, err := FindActive(dir)
	if err != nil {
		t.Fatalf("FindActive failed: %v", err)
	}
	if found.ID != s.ID {
		t.Errorf("expected ID %s, got %s", s.ID, found.ID)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/session/... -v
```

Expected: FAIL - package does not exist

- [ ] **Step 3: Implement session module**

Create `internal/session/session.go`:

```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the session state
type Status string

const (
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusBlocked Status = "blocked"
	StatusFailed  Status = "failed"
)

// Session represents a sandbox execution session
type Session struct {
	ID           string    `json:"id"`
	WorktreePath string    `json:"worktree_path"`
	SpecPath     string    `json:"spec_path"`
	Status       Status    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	ContainerID  string    `json:"container_id,omitempty"`
	LogPath      string    `json:"log_path,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// New creates a new session
func New(worktreePath, specPath string) *Session {
	id := generateID()
	home, _ := os.UserHomeDir()

	return &Session{
		ID:           id,
		WorktreePath: worktreePath,
		SpecPath:     specPath,
		Status:       StatusRunning,
		StartedAt:    time.Now(),
		LogPath:      filepath.Join(home, ".claude", "sandbox-sessions", id+".log"),
	}
}

// Save persists the session state to disk
func (s *Session) Save(worktreePath string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	sessionFile := filepath.Join(worktreePath, "session.json")
	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load reads session state from disk
func Load(worktreePath string) (*Session, error) {
	sessionFile := filepath.Join(worktreePath, "session.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &s, nil
}

// FindActive finds an active session in the worktree
func FindActive(worktreePath string) (*Session, error) {
	s, err := Load(worktreePath)
	if err != nil {
		return nil, err
	}

	if s.Status != StatusRunning {
		return nil, fmt.Errorf("no active session (status: %s)", s.Status)
	}

	return s, nil
}

// Complete marks the session as complete
func (s *Session) Complete(status Status) {
	s.Status = status
	s.CompletedAt = time.Now()
}

// Duration returns how long the session has been running
func (s *Session) Duration() time.Duration {
	end := s.CompletedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(s.StartedAt)
}

// IsActive returns true if the session is still running
func (s *Session) IsActive() bool {
	return s.Status == StatusRunning
}

func generateID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s-%s", date, hex.EncodeToString(bytes)[:6])
}

// EnsureLogDir creates the session log directory
func EnsureLogDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(home, ".claude", "sandbox-sessions")
	return os.MkdirAll(logDir, 0755)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/session/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/
git commit -m "feat: add session module for state management"
```

---

## Phase 8: CLI Commands

### Task 8.1: Implement Init Command

**Files:**
- Create: `internal/cli/init.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write test**

Create `internal/cli/init_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func setupTestRepoForCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"touch", "README.md"},
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup command %v failed: %v", args, err)
		}
	}

	return dir
}

func TestInitCommand(t *testing.T) {
	repo := setupTestRepoForCLI(t)

	cmd := NewRootCommand("test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", repo})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "sandbox/") {
		t.Errorf("expected sandbox branch in output, got: %s", output)
	}
	if !strings.Contains(output, "Worktree ready") {
		t.Errorf("expected 'Worktree ready' in output, got: %s", output)
	}

	// Cleanup: find and remove the worktree
	entries, _ := os.ReadDir(os.TempDir())
	for _, e := range entries {
		if strings.Contains(e.Name(), "-sandbox-") {
			path := os.TempDir() + "/" + e.Name()
			exec.Command("git", "-C", repo, "worktree", "remove", "--force", path).Run()
			os.RemoveAll(path)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cli/... -run TestInitCommand -v
```

Expected: FAIL - init command not defined

- [ ] **Step 3: Implement init command**

Create `internal/cli/init.go`:

```go
package cli

import (
	"fmt"

	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <project-path>",
		Short: "Create a git worktree for sandboxed work",
		Long: `Creates a new git worktree for isolated sandbox work.

The worktree is created with a branch named sandbox/<date>-<hash>.
Use this worktree for planning and spec creation before running
claude-sandbox run.`,
		Args: cobra.ExactArgs(1),
		RunE: runInit,
	}

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	projectPath := args[0]

	if !worktree.IsGitRepo(projectPath) {
		return fmt.Errorf("not a git repository: %s", projectPath)
	}

	cmd.Println("Creating worktree...")

	wt, err := worktree.Create(projectPath)
	if err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	cmd.Printf("  Branch: %s\n", wt.Branch)
	cmd.Printf("  Path:   %s\n", wt.Path)
	cmd.Println()
	cmd.Println("Worktree ready. Next steps:")
	cmd.Printf("  1. cd %s\n", wt.Path)
	cmd.Println("  2. Create your spec (or use Claude to plan)")
	cmd.Println("  3. claude-sandbox run --spec ./path/to/spec")

	return nil
}
```

- [ ] **Step 4: Update root command to add init**

Modify `internal/cli/root.go`:

```go
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

	// Add subcommands
	cmd.AddCommand(newInitCommand())

	return cmd
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/cli/... -v
```

Expected: PASS

- [ ] **Step 6: Test manually**

```bash
make build
./bin/claude-sandbox init --help
```

- [ ] **Step 7: Commit**

```bash
git add internal/cli/init.go internal/cli/init_test.go internal/cli/root.go
git commit -m "feat: add init command for worktree creation"
```

---

### Task 8.2: Implement Run Command

**Files:**
- Create: `internal/cli/run.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Implement run command**

Create `internal/cli/run.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/samueldacanay/claude-sandbox/internal/config"
	"github.com/samueldacanay/claude-sandbox/internal/container"
	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newRunCommand() *cobra.Command {
	var specPath string
	var timeout string
	var retries int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Launch Claude in sandboxed container",
		Long: `Launches Claude Code in an isolated container to implement the specified spec.

The container has quality gates enforced:
  - Build must succeed
  - Lint must pass
  - Tests must pass
  - Security scan must pass
  - Spec coverage verified
  - Commit hygiene checked
  - /review-code must return grade A

COMPLETION.md is written when done (success or blocked).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, specPath, timeout, retries)
		},
	}

	cmd.Flags().StringVar(&specPath, "spec", "", "Path to spec file or directory (required)")
	cmd.Flags().StringVar(&timeout, "timeout", "2h", "Maximum execution time")
	cmd.Flags().IntVar(&retries, "retries", 3, "Max retry attempts per quality gate")
	cmd.MarkFlagRequired("spec")

	return cmd
}

func runRun(cmd *cobra.Command, specPath, timeout string, retries int) error {
	// Resolve spec path
	absSpec, err := filepath.Abs(specPath)
	if err != nil {
		return fmt.Errorf("resolve spec path: %w", err)
	}

	if _, err := os.Stat(absSpec); os.IsNotExist(err) {
		return fmt.Errorf("spec not found: %s", absSpec)
	}

	// Detect worktree
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree: %w", err)
	}

	// Check for image
	if !container.ImageExists(container.DefaultImage) {
		return fmt.Errorf("container image not found: %s\nRun: cd container && ./build.sh --load", container.DefaultImage)
	}

	// Load config
	cfg, err := config.Load(wt.Path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override from flags
	if timeout != "" {
		cfg.Timeout = timeout
	}
	if retries > 0 {
		cfg.Retries = retries
	}

	// Ensure session log directory
	if err := session.EnsureLogDir(); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	// Ensure history volume
	if err := container.EnsureHistoryVolume(); err != nil {
		return fmt.Errorf("create history volume: %w", err)
	}

	// Create session
	sess := session.New(wt.Path, absSpec)
	if err := sess.Save(wt.Path); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	cmd.Println("Starting sandboxed Claude session...")
	cmd.Printf("  Spec:      %s\n", specPath)
	cmd.Printf("  Worktree:  %s\n", wt.Path)
	cmd.Printf("  Container: %s\n", container.DefaultImage)
	cmd.Println()
	cmd.Println("Claude is working. You'll be notified on completion.")
	cmd.Printf("Session log: %s\n", sess.LogPath)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cmd.Println("\nReceived interrupt, stopping...")
		cancel()
	}()

	// Run container
	home, _ := os.UserHomeDir()
	err = container.Run(ctx, container.RunOptions{
		Image:        container.DefaultImage,
		WorktreePath: wt.Path,
		HomeDir:      home,
		SpecPath:     absSpec,
		Timeout:      cfg.Timeout,
		Interactive:  true,
	})

	if err != nil {
		sess.Complete(session.StatusFailed)
		sess.Error = err.Error()
	} else {
		sess.Complete(session.StatusSuccess)
	}

	if saveErr := sess.Save(wt.Path); saveErr != nil {
		cmd.PrintErrf("Warning: failed to save session state: %v\n", saveErr)
	}

	// Fire notification (best effort)
	fireNotification(sess)

	return err
}

func fireNotification(sess *session.Session) {
	// Try to run claude-notify
	// This is best-effort, don't fail if it doesn't work
	_ = sess // TODO: implement notification
}
```

- [ ] **Step 2: Update root command**

Modify `internal/cli/root.go` to add run command:

```go
cmd.AddCommand(newRunCommand())
```

- [ ] **Step 3: Build and verify**

```bash
make build
./bin/claude-sandbox run --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/run.go internal/cli/root.go
git commit -m "feat: add run command for sandboxed execution"
```

---

### Task 8.3: Implement Ship Command

**Files:**
- Create: `internal/cli/ship.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Implement ship command**

Create `internal/cli/ship.go`:

```go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newShipCommand() *cobra.Command {
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
			return runShip(cmd, skipReview, keepWorktree)
		},
	}

	cmd.Flags().BoolVar(&skipReview, "skip-review", false, "Skip COMPLETION.md review prompt")
	cmd.Flags().BoolVar(&keepWorktree, "keep-worktree", false, "Don't clean up worktree after shipping")

	return cmd
}

func runShip(cmd *cobra.Command, skipReview, keepWorktree bool) error {
	// Detect worktree
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree: %w", err)
	}

	// Check for COMPLETION.md
	completionPath := filepath.Join(wt.Path, "COMPLETION.md")
	if _, err := os.Stat(completionPath); os.IsNotExist(err) {
		return fmt.Errorf("COMPLETION.md not found. Run 'claude-sandbox run' first")
	}

	// Read and validate COMPLETION.md
	content, err := os.ReadFile(completionPath)
	if err != nil {
		return fmt.Errorf("read COMPLETION.md: %w", err)
	}

	if !strings.Contains(string(content), "Status: SUCCESS") {
		return fmt.Errorf("COMPLETION.md does not show SUCCESS status. Cannot ship blocked or failed work")
	}

	// Review prompt
	if !skipReview {
		if !promptYesNo(cmd, "Review COMPLETION.md before shipping?", true) {
			return fmt.Errorf("shipping cancelled")
		}

		// Open in editor
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

	// Confirmation
	if !promptYesNo(cmd, "Ship this work?", false) {
		return fmt.Errorf("shipping cancelled")
	}

	cmd.Println("Launching Claude to create PR via /create-pr...")

	// Run Claude with /create-pr skill
	claudeCmd := exec.Command("claude", "--dangerously-skip-permissions", "/create-pr")
	claudeCmd.Dir = wt.Path
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr

	if err := claudeCmd.Run(); err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	// Cleanup prompt
	if !keepWorktree {
		if promptYesNo(cmd, "Clean up worktree?", true) {
			if err := worktree.Remove(wt.Path); err != nil {
				cmd.PrintErrf("Warning: failed to remove worktree: %v\n", err)
			} else {
				cmd.Println("Worktree removed.")
			}
		}
	}

	return nil
}

func promptYesNo(cmd *cobra.Command, question string, defaultYes bool) bool {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}

	cmd.Printf("%s %s ", question, suffix)

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" {
		return defaultYes
	}

	return response == "y" || response == "yes"
}
```

- [ ] **Step 2: Update root command**

Add to `internal/cli/root.go`:

```go
cmd.AddCommand(newShipCommand())
```

- [ ] **Step 3: Build and verify**

```bash
make build
./bin/claude-sandbox ship --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ship.go internal/cli/root.go
git commit -m "feat: add ship command for PR creation"
```

---

### Task 8.4: Implement Utility Commands

**Files:**
- Create: `internal/cli/status.go`
- Create: `internal/cli/logs.go`
- Create: `internal/cli/stop.go`
- Create: `internal/cli/clean.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Implement status command**

Create `internal/cli/status.go`:

```go
package cli

import (
	"fmt"
	"os"

	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of current sandbox session",
		RunE:  runStatus,
	}
	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree")
	}

	sess, err := session.Load(wt.Path)
	if err != nil {
		cmd.Println("No session found in this worktree.")
		return nil
	}

	cmd.Printf("Session: %s\n", sess.ID)
	cmd.Printf("Status:  %s\n", sess.Status)
	cmd.Printf("Started: %s\n", sess.StartedAt.Format("2006-01-02 15:04:05"))
	if !sess.CompletedAt.IsZero() {
		cmd.Printf("Completed: %s\n", sess.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	cmd.Printf("Duration: %s\n", sess.Duration().Round(1e9))
	cmd.Printf("Spec: %s\n", sess.SpecPath)
	cmd.Printf("Log: %s\n", sess.LogPath)

	return nil
}
```

- [ ] **Step 2: Implement logs command**

Create `internal/cli/logs.go`:

```go
package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newLogsCommand() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View session logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd, follow)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}

func runLogs(cmd *cobra.Command, follow bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree")
	}

	sess, err := session.Load(wt.Path)
	if err != nil {
		return fmt.Errorf("no session found")
	}

	if _, err := os.Stat(sess.LogPath); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s", sess.LogPath)
	}

	var tailCmd *exec.Cmd
	if follow {
		tailCmd = exec.Command("tail", "-f", sess.LogPath)
	} else {
		tailCmd = exec.Command("tail", "-100", sess.LogPath)
	}

	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr
	return tailCmd.Run()
}
```

- [ ] **Step 3: Implement stop command**

Create `internal/cli/stop.go`:

```go
package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/samueldacanay/claude-sandbox/internal/session"
	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running sandbox session",
		RunE:  runStop,
	}
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	wt, err := worktree.Detect(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git worktree")
	}

	sess, err := session.FindActive(wt.Path)
	if err != nil {
		return fmt.Errorf("no active session: %w", err)
	}

	if sess.ContainerID != "" {
		cmd.Printf("Stopping container %s...\n", sess.ContainerID[:12])
		stopCmd := exec.Command("docker", "stop", sess.ContainerID)
		_ = stopCmd.Run()
	}

	sess.Complete(session.StatusFailed)
	sess.Error = "stopped by user"
	if err := sess.Save(wt.Path); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	cmd.Println("Session stopped.")
	return nil
}
```

- [ ] **Step 4: Implement clean command**

Create `internal/cli/clean.go`:

```go
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/samueldacanay/claude-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clean [repo-path]",
		Short: "Remove stale sandbox worktrees",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := "."
			if len(args) > 0 {
				repoPath = args[0]
			}
			return runClean(cmd, repoPath, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Remove without confirmation")
	return cmd
}

func runClean(cmd *cobra.Command, repoPath string, force bool) error {
	if !worktree.IsGitRepo(repoPath) {
		return fmt.Errorf("not a git repository: %s", repoPath)
	}

	worktrees, err := worktree.List(repoPath)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	var sandboxWorktrees []worktree.Worktree
	for _, wt := range worktrees {
		if strings.HasPrefix(wt.Branch, "sandbox/") {
			sandboxWorktrees = append(sandboxWorktrees, wt)
		}
	}

	if len(sandboxWorktrees) == 0 {
		cmd.Println("No sandbox worktrees found.")
		return nil
	}

	cmd.Printf("Found %d sandbox worktree(s):\n", len(sandboxWorktrees))
	for _, wt := range sandboxWorktrees {
		cmd.Printf("  - %s (%s)\n", wt.Branch, wt.Path)
	}

	if !force {
		cmd.Print("\nRemove all? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			return nil
		}
	}

	for _, wt := range sandboxWorktrees {
		cmd.Printf("Removing %s...\n", wt.Branch)
		if err := worktree.Remove(wt.Path); err != nil {
			cmd.PrintErrf("  Warning: %v\n", err)
		}
	}

	cmd.Println("Done.")
	return nil
}
```

- [ ] **Step 5: Update root command**

Add all commands to `internal/cli/root.go`:

```go
func NewRootCommand(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "claude-sandbox",
		Short:   "Sandboxed execution environment for Claude Code",
		Long:    `claude-sandbox enables autonomous Claude Code execution in isolated containers with quality gates and external action blocking.`,
		Version: version,
	}

	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newShipCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newLogsCommand())
	cmd.AddCommand(newStopCommand())
	cmd.AddCommand(newCleanCommand())

	return cmd
}
```

- [ ] **Step 6: Build and verify all commands**

```bash
make build
./bin/claude-sandbox --help
./bin/claude-sandbox status --help
./bin/claude-sandbox logs --help
./bin/claude-sandbox stop --help
./bin/claude-sandbox clean --help
```

- [ ] **Step 7: Commit**

```bash
git add internal/cli/
git commit -m "feat: add utility commands (status, logs, stop, clean)"
```

---

## Phase 9: Integration & Documentation

### Task 9.1: Create README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create README**

Create `README.md`:

```markdown
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
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage instructions"
```

---

### Task 9.2: Run Full Test Suite

- [ ] **Step 1: Run all tests**

```bash
make test
```

Expected: All tests pass

- [ ] **Step 2: Run linter**

```bash
make lint
```

Expected: No errors

- [ ] **Step 3: Build final binary**

```bash
make build
./bin/claude-sandbox --version
```

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "chore: final cleanup and verification"
```

---

## Summary

**Total tasks:** 12 tasks across 9 phases

**Key deliverables:**
1. Go CLI with 7 commands (init, run, ship, status, logs, stop, clean)
2. apko container image with full dev toolchain
3. sandbox-guard.sh hook for external action blocking
4. Session state management
5. Quality gate detection and configuration

**Testing approach:**
- Unit tests for all modules (worktree, config, container, session)
- Integration tests for CLI commands
- Manual verification of container and hook behavior

**Dependencies:**
- Go 1.22+
- cobra, viper
- Docker
- apko (for building container image)
