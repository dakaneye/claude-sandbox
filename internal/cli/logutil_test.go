package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLogTail(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		lines    int
		expected string
	}{
		{
			name:     "empty file",
			content:  "",
			lines:    10,
			expected: "",
		},
		{
			name:     "fewer lines than requested",
			content:  "line1\nline2\nline3",
			lines:    10,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "exact lines requested",
			content:  "line1\nline2\nline3",
			lines:    3,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "more lines than requested",
			content:  "line1\nline2\nline3\nline4\nline5",
			lines:    3,
			expected: "line3\nline4\nline5",
		},
		{
			name:     "single line requested from many",
			content:  "line1\nline2\nline3",
			lines:    1,
			expected: "line3",
		},
		{
			name:     "ring buffer wrap around",
			content:  "a\nb\nc\nd\ne\nf\ng",
			lines:    3,
			expected: "e\nf\ng",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with content
			dir := t.TempDir()
			path := filepath.Join(dir, "test.log")

			if tt.content != "" {
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatalf("write test file: %v", err)
				}
			} else {
				// Create empty file
				f, err := os.Create(path)
				if err != nil {
					t.Fatalf("create test file: %v", err)
				}
				f.Close()
			}

			got := readLogTail(path, tt.lines)
			if got != tt.expected {
				t.Errorf("readLogTail() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReadLogTailFileNotFound(t *testing.T) {
	got := readLogTail("/nonexistent/path/file.log", 10)
	if got != "" {
		t.Errorf("readLogTail() = %q, want empty string for missing file", got)
	}
}

func TestReadLogTailLongLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create a file with a very long line (100KB)
	longLine := strings.Repeat("x", 100*1024)
	content := "short1\n" + longLine + "\nshort2"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	got := readLogTail(path, 2)
	expected := longLine + "\nshort2"
	if got != expected {
		t.Errorf("readLogTail() did not handle long line correctly, got length %d", len(got))
	}
}
