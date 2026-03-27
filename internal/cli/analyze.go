package cli

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const analysisPrompt = `Analyze this Claude Code session log. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's currently doing
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Log tail:
`

// analyzeLog uses Claude haiku to analyze the log content.
// Returns empty string if analysis fails (graceful degradation).
// Respects the provided context for cancellation with a 30s timeout.
func analyzeLog(ctx context.Context, logContent string) string {
	if logContent == "" {
		return ""
	}

	prompt := analysisPrompt + logContent

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", prompt)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// claudeAvailable checks if the claude CLI is available.
func claudeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}
