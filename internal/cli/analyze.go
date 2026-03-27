package cli

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

//nolint:unused // Will be used by status command
const analysisPrompt = `Analyze this Claude Code session log. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's currently doing
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Log tail:
`

// analyzeLog uses Claude haiku to analyze the log content.
// Returns empty string if analysis fails (graceful degradation).
//
//nolint:unused // Will be used by status command
func analyzeLog(logContent string) string {
	if logContent == "" {
		return ""
	}

	prompt := analysisPrompt + logContent

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", prompt)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// claudeAvailable checks if the claude CLI is available.
//
//nolint:unused // Will be used by status command
func claudeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}
