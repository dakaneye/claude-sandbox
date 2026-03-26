package container

import (
	"os"
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
	// Note: settings.json, hooks, skills NOT mounted - container uses pre-baked config
	requiredSources := []string{
		"/Users/test/.claude/commands",
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
		if strings.Contains(m.Source, ".claude/commands") && !m.ReadOnly {
			t.Error("commands should be read-only")
		}
	}
}

func TestBuildRunArgs(t *testing.T) {
	opts := RunOptions{
		Image:        "claude-sandbox:latest",
		WorktreePath: "/tmp/worktree",
		HomeDir:      "/Users/test",
		SpecPath:     "/tmp/worktree/spec.md",
		Interactive:  true,
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

	// Should have -it for interactive mode
	foundIT := false
	for _, arg := range args {
		if arg == "-it" {
			foundIT = true
			break
		}
	}
	if !foundIT {
		t.Error("missing -it flags for interactive mode")
	}

	// Should pass ANTHROPIC_API_KEY via environment inheritance (no value in args)
	for i, arg := range args {
		if arg == "-e" && i+1 < len(args) && args[i+1] == "ANTHROPIC_API_KEY" {
			return // Found it
		}
	}
	t.Error("missing ANTHROPIC_API_KEY environment variable passthrough")
}

func TestBuildRunArgsNonInteractive(t *testing.T) {
	opts := RunOptions{
		Image:        "claude-sandbox:latest",
		WorktreePath: "/tmp/worktree",
		HomeDir:      "/Users/test",
		SpecPath:     "/tmp/worktree/spec.md",
		Interactive:  false,
	}

	args := BuildRunArgs(opts)

	// Should NOT have -it for non-interactive mode
	for _, arg := range args {
		if arg == "-it" {
			t.Error("-it should not be present in non-interactive mode")
		}
	}
}

func TestMountToDockerArgs(t *testing.T) {
	tests := []struct {
		name     string
		mount    Mount
		expected []string
	}{
		{
			name: "read-only mount",
			mount: Mount{
				Source:   "/host/path",
				Target:   "/container/path",
				ReadOnly: true,
			},
			expected: []string{"-v", "/host/path:/container/path:ro"},
		},
		{
			name: "read-write mount",
			mount: Mount{
				Source:   "/host/path",
				Target:   "/container/path",
				ReadOnly: false,
			},
			expected: []string{"-v", "/host/path:/container/path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.mount.ToDockerArgs()
			if len(args) != len(tt.expected) {
				t.Errorf("expected %d args, got %d", len(tt.expected), len(args))
				return
			}
			for i, arg := range args {
				if arg != tt.expected[i] {
					t.Errorf("arg %d: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

func TestBuildClaudeCommand(t *testing.T) {
	cmd := buildClaudeCommand("/workspace/spec.md")

	if !strings.Contains(cmd, "/workspace/spec.md") {
		t.Error("command should contain spec path")
	}
	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Error("command should include --dangerously-skip-permissions")
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with spaces", "'with spaces'"},
		{"with'quote", "'with'\\''quote'"},
		{"$(whoami)", "'$(whoami)'"},
		{"`id`", "'`id`'"},
		{"$HOME", "'$HOME'"},
		{"a;rm -rf /", "'a;rm -rf /'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellEscape(tt.input)
			if got != tt.expected {
				t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHostToContainerPath(t *testing.T) {
	tests := []struct {
		name         string
		hostPath     string
		worktreePath string
		expected     string
	}{
		{
			name:         "file in worktree root",
			hostPath:     "/tmp/worktree/spec.md",
			worktreePath: "/tmp/worktree",
			expected:     "/workspace/spec.md",
		},
		{
			name:         "file in subdirectory",
			hostPath:     "/tmp/worktree/docs/specs/feature.md",
			worktreePath: "/tmp/worktree",
			expected:     "/workspace/docs/specs/feature.md",
		},
		{
			name:         "worktree root itself",
			hostPath:     "/tmp/worktree",
			worktreePath: "/tmp/worktree",
			expected:     "/workspace",
		},
		{
			name:         "path outside worktree",
			hostPath:     "/other/path/file.md",
			worktreePath: "/tmp/worktree",
			expected:     "/other/path/file.md",
		},
		{
			name:         "worktree with trailing slash",
			hostPath:     "/tmp/worktree/spec.md",
			worktreePath: "/tmp/worktree/",
			expected:     "/workspace/spec.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize worktree path (remove trailing slash for consistent behavior)
			worktree := strings.TrimSuffix(tt.worktreePath, "/")
			got := hostToContainerPath(tt.hostPath, worktree)
			if got != tt.expected {
				t.Errorf("hostToContainerPath(%q, %q) = %q, want %q",
					tt.hostPath, tt.worktreePath, got, tt.expected)
			}
		})
	}
}

func TestRunOptions_MissingAPIKey(t *testing.T) {
	// Save and clear env
	orig := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("ANTHROPIC_API_KEY", orig)
		}
	}()

	opts := RunOptions{
		WorktreePath: "/tmp/worktree",
		SpecPath:     "/tmp/worktree/spec.md",
		// APIKey intentionally empty
	}

	err := Run(t.Context(), opts)
	if err == nil {
		t.Error("expected error when ANTHROPIC_API_KEY not set")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("error should mention ANTHROPIC_API_KEY, got: %v", err)
	}
}
