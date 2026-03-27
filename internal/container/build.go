package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// writeConfigs writes embedded config files to a temporary directory.
// Returns paths to apko.yaml and prebake.dockerfile.
func writeConfigs(tmpDir string) (apkoPath, dockerfilePath string, err error) {
	apkoPath = filepath.Join(tmpDir, "apko.yaml")
	if err := os.WriteFile(apkoPath, apkoConfig, 0644); err != nil {
		return "", "", fmt.Errorf("write apko.yaml: %w", err)
	}

	dockerfilePath = filepath.Join(tmpDir, "prebake.dockerfile")
	if err := os.WriteFile(dockerfilePath, prebakeDockerfile, 0644); err != nil {
		return "", "", fmt.Errorf("write prebake.dockerfile: %w", err)
	}

	return apkoPath, dockerfilePath, nil
}
