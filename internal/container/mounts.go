// Package container handles Docker container operations for sandbox execution.
package container

import (
	"os"
	"path/filepath"
)

// Mount represents a Docker volume mount.
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// MountOptions configures mount generation.
type MountOptions struct {
	WorktreePath string
	HomeDir      string
}

// BuildMounts generates the volume mounts for the sandbox container.
// Only mounts paths that exist on the host to avoid Docker creating empty
// directories in the user's home.
func BuildMounts(opts MountOptions) []Mount {
	home := opts.HomeDir

	// Workspace is always mounted (read-write)
	mounts := []Mount{
		{
			Source:   opts.WorktreePath,
			Target:   "/workspace",
			ReadOnly: false,
		},
	}

	// Optional read-only mounts — only included if they exist on the host.
	// Note: settings.json, hooks, and skills are NOT mounted; the container
	// uses pre-baked versions to ensure consistent behavior.
	optional := []Mount{
		{
			Source:   filepath.Join(home, ".claude", "commands"),
			Target:   "/home/claude/.claude/commands",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".gitconfig"),
			Target:   "/home/claude/.gitconfig",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".ssh"),
			Target:   "/home/claude/.ssh",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".config", "gh"),
			Target:   "/home/claude/.config/gh",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".config", "chainctl"),
			Target:   "/home/claude/.config/chainctl",
			ReadOnly: true,
		},
	}

	for _, m := range optional {
		if _, err := os.Stat(m.Source); err == nil {
			mounts = append(mounts, m)
		}
	}

	return mounts
}

// ToDockerArgs converts a mount to Docker -v arguments.
func (m Mount) ToDockerArgs() []string {
	mountSpec := m.Source + ":" + m.Target
	if m.ReadOnly {
		mountSpec += ":ro"
	}
	return []string{"-v", mountSpec}
}
