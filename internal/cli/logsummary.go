package cli

import (
	"bufio"
	"encoding/json"
	"os"
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
