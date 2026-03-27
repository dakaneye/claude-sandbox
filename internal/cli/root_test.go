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
			if sub.Name() == name {
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
			if sub.Name() == name {
				t.Errorf("removed command %q still exists", name)
			}
		}
	}
}
