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
		// Read-only config mounts
		{
			Source:   filepath.Join(home, ".claude", "settings.json"),
			Target:   "/home/claude/.claude/settings.json",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".claude", "hooks"),
			Target:   "/home/claude/.claude/hooks",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".claude", "commands"),
			Target:   "/home/claude/.claude/commands",
			ReadOnly: true,
		},
		{
			Source:   filepath.Join(home, ".claude", "skills"),
			Target:   "/home/claude/.claude/skills",
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
