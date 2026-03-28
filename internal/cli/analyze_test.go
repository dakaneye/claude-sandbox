package cli

import "testing"

func TestAnalyzeLog_EmptyInput(t *testing.T) {
	analysis, reason := analyzeLog(t.Context(), "")
	if analysis != "" {
		t.Errorf("analysis = %q, want empty for empty input", analysis)
	}
	if reason != "no log data" {
		t.Errorf("reason = %q, want %q", reason, "no log data")
	}
}
