package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// LogSummary holds structured metrics extracted from a stream-json log.
type LogSummary struct {
	ElapsedTime  time.Duration
	TotalTools   int
	ToolCounts   map[string]int
	LastTool     string
	LastToolDesc string
	LastToolTime time.Time
	TaskProgress []string
	GateMentions map[string]bool
}

// logEvent is the minimal JSON structure we extract from each line.
type logEvent struct {
	Type        string      `json:"type"`
	Subtype     string      `json:"subtype"`
	Description string      `json:"description"`
	Message     *logMessage `json:"message"`
	Timestamp   string      `json:"timestamp"`
}

type logMessage struct {
	Content []logContent `json:"content"`
}

type logContent struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolInput struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

const maxTaskProgress = 3

var gatePatterns = map[string][]string{
	"build":       {"make build", "go build"},
	"lint":        {"golangci-lint", "lint"},
	"test":        {"go test"},
	"review-code": {"review-code", "/review"},
}

func parseLogEvents(path string, startedAt time.Time) (*LogSummary, error) {
	summary := &LogSummary{
		ElapsedTime:  time.Since(startedAt),
		ToolCounts:   make(map[string]int),
		GateMentions: make(map[string]bool),
	}

	file, err := os.Open(path)
	if err != nil {
		return summary, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		var event logEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		switch event.Type {
		case "assistant":
			processAssistantEvent(summary, &event)
		case "system":
			processSystemEvent(summary, &event)
		case "user":
			processUserEvent(summary, &event)
		}
	}

	return summary, nil
}

func processAssistantEvent(s *LogSummary, event *logEvent) {
	if event.Message == nil {
		return
	}
	for _, content := range event.Message.Content {
		if content.Type != "tool_use" {
			continue
		}
		s.TotalTools++
		s.ToolCounts[content.Name]++
		s.LastTool = content.Name

		var input toolInput
		if err := json.Unmarshal(content.Input, &input); err == nil {
			if input.Description != "" {
				s.LastToolDesc = input.Description
			} else if input.Command != "" {
				cmd := input.Command
				if len(cmd) > 80 {
					cmd = cmd[:77] + "..."
				}
				s.LastToolDesc = cmd
			}
		}

		if content.Name == "Bash" {
			checkGates(s, content.Input)
		}
	}
}

func processSystemEvent(s *LogSummary, event *logEvent) {
	if event.Subtype != "task_progress" || event.Description == "" {
		return
	}
	if len(s.TaskProgress) >= maxTaskProgress {
		s.TaskProgress = s.TaskProgress[1:]
	}
	s.TaskProgress = append(s.TaskProgress, event.Description)
}

func processUserEvent(s *LogSummary, event *logEvent) {
	if event.Timestamp == "" {
		return
	}
	t, err := time.Parse(time.RFC3339Nano, event.Timestamp)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000Z", event.Timestamp)
		if err != nil {
			return
		}
	}
	s.LastToolTime = t
}

// formatSummary produces a condensed text summary for haiku analysis.
func formatSummary(s *LogSummary) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Session: %s elapsed, %d tool calls\n", s.ElapsedTime.Round(time.Second), s.TotalTools)

	// Tool counts sorted by frequency
	if len(s.ToolCounts) > 0 {
		type toolCount struct {
			name  string
			count int
		}
		sorted := make([]toolCount, 0, len(s.ToolCounts))
		for name, count := range s.ToolCounts {
			sorted = append(sorted, toolCount{name, count})
		}
		slices.SortFunc(sorted, func(a, b toolCount) int {
			return b.count - a.count
		})
		b.WriteString("Tools: ")
		for i, tc := range sorted {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s(%d)", tc.name, tc.count)
		}
		b.WriteString("\n")
	}

	// Last tool
	if s.LastTool != "" {
		b.WriteString("Last: ")
		b.WriteString(s.LastTool)
		if s.LastToolDesc != "" {
			fmt.Fprintf(&b, " %q", s.LastToolDesc)
		}
		if !s.LastToolTime.IsZero() {
			ago := time.Since(s.LastToolTime).Round(time.Second)
			fmt.Fprintf(&b, " (%s ago)", ago)
		}
		b.WriteString("\n")
	}

	// Task progress
	if len(s.TaskProgress) > 0 {
		b.WriteString("Recent progress:\n")
		for _, desc := range s.TaskProgress {
			fmt.Fprintf(&b, "- %s\n", desc)
		}
	}

	// Gates
	gates := []string{"build", "lint", "test", "review-code"}
	b.WriteString("Gates: ")
	for i, gate := range gates {
		if i > 0 {
			b.WriteString(" | ")
		}
		if s.GateMentions[gate] {
			fmt.Fprintf(&b, "%s seen", gate)
		} else {
			fmt.Fprintf(&b, "%s ?", gate)
		}
	}
	b.WriteString("\n")

	return b.String()
}

// formatFallback produces a raw metrics display with a warning when haiku analysis fails.
func formatFallback(s *LogSummary, reason string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "⚠ Analysis unavailable: %s\n\n", reason)

	fmt.Fprintf(&b, "Progress: %d tool calls over %s\n", s.TotalTools, s.ElapsedTime.Round(time.Second))

	if len(s.ToolCounts) > 0 {
		type toolCount struct {
			name  string
			count int
		}
		sorted := make([]toolCount, 0, len(s.ToolCounts))
		for name, count := range s.ToolCounts {
			sorted = append(sorted, toolCount{name, count})
		}
		slices.SortFunc(sorted, func(a, b toolCount) int {
			return b.count - a.count
		})
		b.WriteString("Tools: ")
		for i, tc := range sorted {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s(%d)", tc.name, tc.count)
		}
		b.WriteString("\n")
	}

	if s.LastTool != "" {
		fmt.Fprintf(&b, "Last: %s", s.LastTool)
		if s.LastToolDesc != "" {
			fmt.Fprintf(&b, " %q", s.LastToolDesc)
		}
		b.WriteString("\n")
	}

	gates := []string{"build", "lint", "test", "review-code"}
	b.WriteString("Gates: ")
	for i, gate := range gates {
		if i > 0 {
			b.WriteString(" | ")
		}
		if s.GateMentions[gate] {
			b.WriteString(gate + " seen")
		} else {
			b.WriteString(gate + " ?")
		}
	}
	b.WriteString("\n")

	return b.String()
}

func checkGates(s *LogSummary, raw json.RawMessage) {
	var input toolInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return
	}
	combined := strings.ToLower(input.Command + " " + input.Description)
	for gate, patterns := range gatePatterns {
		for _, pattern := range patterns {
			if strings.Contains(combined, pattern) {
				s.GateMentions[gate] = true
				break
			}
		}
	}
}
