// Package container handles Docker container operations for sandbox execution.
package container

import (
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
func BuildMounts(opts MountOptions) []Mount {
	home := opts.HomeDir

	mounts := []Mount{
		// Note: settings.json NOT mounted - container uses pre-baked settings
		// that skip onboarding and have permissions pre-configured
		//
		// Note: hooks NOT mounted - container should run autonomously
		// without host-specific hooks that might interfere
		//
		// Note: skills NOT mounted - container uses pre-baked skills
		// This ensures /review-code is always available for quality gates
		//
		// Commands are mounted so user can use custom slash commands
		{
			Source:   filepath.Join(home, ".claude", "commands"),
			Target:   "/home/claude/.claude/commands",
			ReadOnly: true,
		},
		// Plugins mounted from host (superpowers, etc.)
		{
			Source:   filepath.Join(home, ".claude", "plugins"),
			Target:   "/home/claude/.claude/plugins",
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
		// Read-write mounts
		{
			Source:   opts.WorktreePath,
			Target:   "/workspace",
			ReadOnly: false,
		},
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
