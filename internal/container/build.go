package container

import (
	"fmt"
	"os/exec"
)

// CheckDocker verifies Docker is running.
func CheckDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running. Start Docker and try again.")
	}
	return nil
}

// CheckApko verifies apko is installed.
func CheckApko() error {
	cmd := exec.Command("apko", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apko not found. Install with: brew install apko (macOS) or go install chainguard.dev/apko@latest")
	}
	return nil
}
