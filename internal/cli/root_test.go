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
