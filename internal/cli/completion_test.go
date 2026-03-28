package cli

import (
	"testing"

	"github.com/dakaneye/claude-sandbox/internal/state"
)

func TestParseCompletionStatus(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected state.Status
	}{
		{
			name:     "strict SUCCESS",
			content:  "STATUS: SUCCESS\n\nAll gates passed.",
			expected: state.StatusSuccess,
		},
		{
			name:     "strict BLOCKED",
			content:  "STATUS: BLOCKED\n\nLint failures.",
			expected: state.StatusBlocked,
		},
		{
			name:     "strict FAILED",
			content:  "STATUS: FAILED\n\nBuild broken.",
			expected: state.StatusFailed,
		},
		{
			name:     "strict lowercase",
			content:  "status: success\n\nDone.",
			expected: state.StatusSuccess,
		},
		{
			name:     "strict with whitespace",
			content:  "  Status:  SUCCESS  \n\nDone.",
			expected: state.StatusSuccess,
		},
		{
			name:     "strict complete maps to success",
			content:  "Status: Complete\n\nDone.",
			expected: state.StatusSuccess,
		},
		{
			name:     "fuzzy markdown bold status",
			content:  "# Completion\n\n**Status**: Complete — Grade A\n\nDetails...",
			expected: state.StatusSuccess,
		},
		{
			name:     "fuzzy success in body",
			content:  "# Result\n\nAll quality gates passed successfully.\n\nGrade A.",
			expected: state.StatusSuccess,
		},
		{
			name:     "fuzzy blocked",
			content:  "# Result\n\nCould not proceed, blocked by lint failures.",
			expected: state.StatusBlocked,
		},
		{
			name:     "blocked beats success",
			content:  "Blocked from reaching success.",
			expected: state.StatusBlocked,
		},
		{
			name:     "old format still works",
			content:  "## Status: SUCCESS\n\nImplemented feature.",
			expected: state.StatusSuccess,
		},
		{
			name:     "no match defaults to failed",
			content:  "# Some random markdown\n\nNo status here.",
			expected: state.StatusFailed,
		},
		{
			name:     "empty content",
			content:  "",
			expected: state.StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCompletionStatus(tt.content)
			if got != tt.expected {
				t.Errorf("parseCompletionStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}
