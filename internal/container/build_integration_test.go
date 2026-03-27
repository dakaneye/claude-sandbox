//go:build integration

package container

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestBuildIntegration(t *testing.T) {
	// Skip if docker is not available
	if err := CheckDocker(); err != nil {
		t.Skipf("Docker not available: %v", err)
	}

	// Skip if apko is not available
	if err := CheckApko(); err != nil {
		t.Skipf("apko not available: %v", err)
	}

	// Remove existing image to test fresh build
	exec.Command("docker", "rmi", "-f", DefaultImage).Run()
	exec.Command("docker", "rmi", "-f", "claude-sandbox:base-amd64").Run()
	exec.Command("docker", "rmi", "-f", "claude-sandbox:base-arm64").Run()

	// Build
	var buf bytes.Buffer
	if err := Build(&buf, true); err != nil {
		t.Fatalf("Build: %v\nOutput:\n%s", err, buf.String())
	}

	// Verify image exists
	if !ImageExists(DefaultImage) {
		t.Error("Image does not exist after build")
	}
}
