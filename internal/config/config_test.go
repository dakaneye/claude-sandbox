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

func TestGetGateCommand_ConfigOverride(t *testing.T) {
	dir := t.TempDir()

	// Create a Go project
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create config with custom build command
	configContent := `
gates:
  build: "custom build command"
`
	configPath := filepath.Join(dir, ".claude-sandbox.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Config override should take precedence
	cmd := cfg.GetGateCommand(dir, "build")
	if cmd != "custom build command" {
		t.Errorf("expected config override, got %q", cmd)
	}

	// Non-overridden gate should use auto-detection
	lintCmd := cfg.GetGateCommand(dir, "lint")
	if lintCmd != "golangci-lint run ./..." {
		t.Errorf("expected auto-detected lint, got %q", lintCmd)
	}
}

func TestDetectGateCommand_UnknownGate(t *testing.T) {
	dir := t.TempDir()

	cmd := DetectGateCommand(dir, "unknown")
	if cmd != "" {
		t.Errorf("expected empty string for unknown gate, got %q", cmd)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()

	// Write invalid YAML
	configPath := filepath.Join(dir, ".claude-sandbox.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}
