package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CheckDocker verifies Docker is running.
func CheckDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not running; start Docker and try again")
	}
	return nil
}

// CheckApko verifies apko is installed.
func CheckApko() error {
	cmd := exec.Command("apko", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apko not found; install with: brew install apko (macOS) or go install chainguard.dev/apko@latest")
	}
	return nil
}

// currentArch returns the current architecture in Docker/apko format.
func currentArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "amd64"
	}
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

// Build builds the sandbox container image.
// If force is false and image exists, returns nil immediately.
// Output is written to w (can be os.Stdout for progress).
func Build(w io.Writer, force bool) error {
	// Check dependencies
	if err := CheckDocker(); err != nil {
		return err
	}
	if err := CheckApko(); err != nil {
		return err
	}

	// Check if image exists (unless force)
	if !force && ImageExists(DefaultImage) {
		fmt.Fprintln(w, "Image claude-sandbox:latest already exists. Use --force to rebuild.")
		return nil
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "claude-sandbox-build-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write configs
	apkoPath, dockerfilePath, err := writeConfigs(tmpDir)
	if err != nil {
		return err
	}

	arch := currentArch()
	tarPath := filepath.Join(tmpDir, "base.tar")
	baseImage := fmt.Sprintf("claude-sandbox:base-%s", arch)

	// Run apko build
	fmt.Fprintln(w, "Building base image with apko...")
	apkoCmd := exec.Command("apko", "build", apkoPath, "claude-sandbox:base", tarPath, "--arch", arch)
	apkoCmd.Stdout = w
	apkoCmd.Stderr = w
	if err := apkoCmd.Run(); err != nil {
		return fmt.Errorf("apko build: %w", err)
	}

	// Load into Docker
	fmt.Fprintln(w, "Loading base image into Docker...")
	loadCmd := exec.Command("docker", "load", "-i", tarPath)
	loadCmd.Stdout = w
	loadCmd.Stderr = w
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("docker load: %w", err)
	}

	// Run docker build for prebake
	fmt.Fprintln(w, "Pre-baking Claude Code into image...")
	buildCmd := exec.Command("docker", "build",
		"-t", DefaultImage,
		"--build-arg", "BASE_IMAGE="+baseImage,
		"-f", dockerfilePath,
		tmpDir,
	)
	buildCmd.Stdout = w
	buildCmd.Stderr = w
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	fmt.Fprintln(w, "Image built: claude-sandbox:latest")
	return nil
}
