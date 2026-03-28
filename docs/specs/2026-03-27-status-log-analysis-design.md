# Status Command Log Analysis Fix

## Problem

The `status` command crashes when a session is running. Root cause: `readLogTail` reads 500 lines of `stream-json` output (each line can be thousands of characters), creating a multi-megabyte string that's passed directly to `claude -p --model haiku`. This causes memory explosion and context overflow, killing the process.

**Observed behavior:**
```
$ claude-sandbox status
[1]  93840 killed  claude-sandbox status
```

**Expected behavior:** Show progress estimate without crashing.

## Solution

Parse the JSON stream logs to extract structured progress metrics, then send a condensed ~500 byte summary to haiku instead of raw log content.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Log File       │────▶│  parseLogEvents  │────▶│  LogSummary     │
│  (stream-json)  │     │  (new function)  │     │  (struct)       │
└─────────────────┘     └──────────────────┘     └────────┬────────┘
                                                          │
                                                          ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Haiku Output   │◀────│  analyzeLog      │◀────│  formatSummary  │
│  (2-3 lines)    │     │  (updated)       │     │  (new function) │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

### Files Changed

| File | Change |
|------|--------|
| `internal/cli/logsummary.go` | New file: JSON parsing and summary generation |
| `internal/cli/logsummary_test.go` | New file: Unit tests |
| `internal/cli/analyze.go` | Update to accept `LogSummary` instead of raw string |
| `internal/cli/status.go` | Update to use new parsing flow |
| `internal/cli/logutil.go` | Remove `readLogTail` (no longer needed) |
| `test/e2e/workflow_test.sh` | New file: E2E workflow test |
| `testdata/session-log.json` | New file: Recorded session for testing |

## Data Structures

### LogSummary

```go
type LogSummary struct {
    ElapsedTime   time.Duration
    TotalTools    int
    ToolCounts    map[string]int  // {"Bash": 15, "Read": 20, "Edit": 8}
    LastTool      string          // "Bash"
    LastToolDesc  string          // "go test ./..."
    LastToolTime  time.Time
    TaskProgress  []string        // Last 3 task_progress descriptions
    GateMentions  map[string]bool // {"build": true, "lint": true, "test": false}
}
```

### JSON Parsing

Function signature:
```go
func parseLogEvents(path string, startedAt time.Time) (*LogSummary, error)
```

**Parsing rules:**
- Read file line-by-line (memory bounded)
- Parse each line as JSON, extract only needed fields
- Track tool counts from `message.content[].name`
- Capture last 3 `task_progress` descriptions from `subtype` field
- Detect gate mentions by keyword matching in Bash commands/descriptions:
  - `build`, `make`, `go build` → build gate
  - `lint`, `golangci-lint` → lint gate
  - `test`, `go test` → test gate
  - `review-code`, `/review` → review gate
- Skip malformed lines, continue with partial data

## Summary Format

### formatSummary Output

```
Session: 12m elapsed, 47 tool calls
Tools: Bash(15), Read(20), Edit(8), Task(4)
Last: Bash "go test ./..." (15s ago)
Recent progress:
- Running tests in internal/cli
- Fixing lint errors
Gates detected: build ✓, lint ✓, test (active), review-code (not seen)
```

### Haiku Prompt

```go
const analysisPrompt = `Analyze this Claude Code session summary. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's likely doing now
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Summary:
`
```

## Error Handling

Graceful degradation chain with obvious failure indicators:

| Condition | Behavior |
|-----------|----------|
| Log file missing/empty | "Execution in progress. No log data yet." |
| JSON parse errors | Skip malformed lines, continue with partial data |
| No events extracted | "Execution in progress." (no haiku call) |
| Claude CLI unavailable | Show raw metrics with warning |
| Haiku timeout/error | Show raw metrics with warning |

### Degraded Output Format

When haiku analysis fails, show clear indicator:

```
⚠ Analysis unavailable: claude CLI timeout

Progress: 47 tool calls over 12m
Tools: Bash(15), Read(20), Edit(8)
Last: Bash "go test ./..." (15s ago)
Gates: build ✓ | lint ✓ | test ⋯ | review ?
```

Reasons shown:
- `claude CLI not found`
- `claude CLI timeout`
- `no log data`
- `log parse error`

## Testing

### Unit Tests

| Test | Input | Validates |
|------|-------|-----------|
| `TestParseLogEvents_Empty` | Empty file | Returns zero-value summary, no error |
| `TestParseLogEvents_MalformedJSON` | Mix of valid/invalid lines | Skips bad lines, extracts good ones |
| `TestParseLogEvents_ToolCounts` | Sample stream-json | Correct tool counting |
| `TestParseLogEvents_GateDetection` | Bash commands with gate keywords | Gate map populated correctly |
| `TestParseLogEvents_TaskProgress` | System messages | Captures last 3 descriptions |
| `TestFormatSummary` | LogSummary struct | Produces expected text format |

### E2E Workflow Test

```bash
#!/bin/bash
# test/e2e/workflow_test.sh
set -e

PLAN="Create a file hello.txt containing 'Hello World'. Run: echo done."

# 1. Spec
claude-sandbox spec --name e2e-test --prompt "$PLAN"

# 2. Execute (with timeout)
timeout 120 claude-sandbox execute --session e2e-test

# 3. Status (should show success, not crash)
claude-sandbox status --session e2e-test | grep -q "completed successfully"

# 4. Verify worktree has COMPLETION.md (get path from session state file)
SESSION_DIR="$(git rev-parse --show-toplevel)/.claude-sandbox/sessions"
WORKTREE=$(jq -r .worktree_path "$SESSION_DIR/e2e-test.json")
test -f "$WORKTREE/COMPLETION.md"

# 5. Clean
claude-sandbox clean --session e2e-test --force

echo "✓ E2E workflow passed"
```

### E2E Scope

- `spec` → `execute` → `status` → `clean` ✓
- `ship` excluded — creates real PRs, would pollute repos with test garbage

### Recorded Session Playback

- Capture real `stream-json` output from a successful session
- Store as `testdata/session-log.json`
- Test status parsing against known-good real output
- Deterministic, no API costs, catches format drift

## Success Criteria

1. `claude-sandbox status` does not crash when session is running
2. Shows meaningful progress estimate when haiku analysis succeeds
3. Shows raw metrics with clear warning when analysis fails
4. Memory usage bounded regardless of log size
5. E2E workflow test passes
