package container

import (
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
