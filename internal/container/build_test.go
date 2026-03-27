package container

import (
	"os"
	"testing"
)

func TestCheckDocker(t *testing.T) {
	// This test verifies CheckDocker returns nil when docker is available.
	// Skip if docker is not installed (CI environments without docker).
	err := CheckDocker()
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
}

func TestCheckApko(t *testing.T) {
	// This test verifies CheckApko returns nil when apko is available.
	// Skip if apko is not installed.
	err := CheckApko()
	if err != nil {
		t.Skipf("apko not available: %v", err)
	}
}

func TestWriteConfigs(t *testing.T) {
	tmpDir := t.TempDir()

	apkoPath, dockerfilePath, err := writeConfigs(tmpDir)
	if err != nil {
		t.Fatalf("writeConfigs: %v", err)
	}

	// Verify apko.yaml was written
	if _, err := os.Stat(apkoPath); err != nil {
		t.Errorf("apko.yaml not created: %v", err)
	}

	// Verify prebake.dockerfile was written
	if _, err := os.Stat(dockerfilePath); err != nil {
		t.Errorf("prebake.dockerfile not created: %v", err)
	}

	// Verify content is non-empty
	apkoContent, _ := os.ReadFile(apkoPath)
	if len(apkoContent) == 0 {
		t.Error("apko.yaml is empty")
	}

	dockerContent, _ := os.ReadFile(dockerfilePath)
	if len(dockerContent) == 0 {
		t.Error("prebake.dockerfile is empty")
	}
}

func TestCurrentArch(t *testing.T) {
	arch := currentArch()
	if arch != "amd64" && arch != "arm64" {
		t.Errorf("unexpected arch: %s", arch)
	}
}
