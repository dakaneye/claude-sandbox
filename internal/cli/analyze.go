package cli

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const analysisPrompt = `Analyze this Claude Code session summary. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's likely doing now
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Summary:
`

// analyzeLog uses Claude haiku to analyze the summary content.
// Returns (analysis, "") on success, or ("", reason) on failure.
// Respects the provided context for cancellation with a 30s timeout.
func analyzeLog(ctx context.Context, summary string) (string, string) {
	if summary == "" {
		return "", "no log data"
	}

	prompt := analysisPrompt + summary

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku", prompt)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", "claude CLI timeout"
		}
		return "", "claude CLI error"
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", "empty analysis response"
	}
	return result, ""
}

// claudeAvailable checks if the claude CLI is available.
func claudeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	return cmd.Run() == nil
}
