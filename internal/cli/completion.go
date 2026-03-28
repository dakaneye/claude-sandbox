package cli

import (
	"strings"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

// parseCompletionStatus determines session status from COMPLETION.md content.
// Strict pass: looks for a line starting with "status:" (case-insensitive).
// Fuzzy fallback: scans for keywords. Priority: blocked > success > failed.
func parseCompletionStatus(content string) state.Status {
	// Strict pass: find a line starting with "status:"
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Strip markdown bold markers
		trimmed = strings.ReplaceAll(trimmed, "**", "")
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, "status:") {
			continue
		}
		value := strings.TrimSpace(trimmed[len("status:"):])
		value = strings.ToLower(value)
		// Strip trailing markdown (e.g., "Complete — Grade A" -> "complete")
		value = strings.SplitN(value, "—", 2)[0]
		value = strings.SplitN(value, "-", 2)[0]
		value = strings.TrimSpace(value)

		switch {
		case value == "success" || value == "complete":
			return state.StatusSuccess
		case value == "blocked":
			return state.StatusBlocked
		case value == "failed":
			return state.StatusFailed
		}
	}

	// Fuzzy fallback: keyword scan with priority
	lower := strings.ToLower(content)
	hasBlocked := strings.Contains(lower, "blocked")
	hasSuccess := strings.Contains(lower, "success") || strings.Contains(lower, "complete")

	if hasBlocked {
		return state.StatusBlocked
	}
	if hasSuccess {
		return state.StatusSuccess
	}
	return state.StatusFailed
}
