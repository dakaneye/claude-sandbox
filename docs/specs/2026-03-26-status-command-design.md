# Status Command Enhancement Design

## Overview

Enhance the `claude-sandbox status` command to provide AI-powered progress analysis of running sandbox sessions.

## Problem

Currently, `status` shows only basic metadata (session ID, timestamps, duration). When a session runs for 10+ minutes, users have no insight into what Claude is doing or how far along it is.

## Solution

Two changes:
1. **Log capture**: Stream container output to log file during `run`
2. **AI analysis**: Use Claude (haiku) to analyze log and summarize progress

## Design

### Log Capture (run command)

- Create log file at `session.LogPath` before starting container
- Use `io.MultiWriter` to write container stdout/stderr to log file
- Spinner continues on terminal showing elapsed time
- Log captures full Claude transcript

### Status Command

1. Load session from worktree
2. Check if log file exists with content
3. Read last 500 lines of log (truncate if larger)
4. Call `claude -p --model haiku` with analysis prompt
5. Display session info + Claude's summary

### Output Format

```
Session: 2026-03-26-abc123
Status:  running
Elapsed: 12m34s

⠋ Analyzing...

~60% - Running /review-code
Iterating on code review feedback. Fixing linting issues
from second review pass. Build and tests passing.
```

### Prompt to Haiku

```
Analyze this Claude Code session log. Respond in 2-3 lines:
- Current phase (implementing, testing, reviewing, fixing, etc.)
- What it's currently doing
- Rough % estimate to completion (quality gates: build, lint, test, /review-code grade A)

Log tail:
<log content>
```

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| No session in worktree | "No session found in this worktree." |
| Session completed | Show final status, skip analysis |
| Log file missing/empty | Show session info + "Log not available yet" |
| Claude CLI not found | Fall back to basic status |
| Haiku call fails | Show session info + "Could not analyze log" |

## Files to Change

- `internal/cli/run.go` - Create log file, pass to container.Run via new LogWriter option
- `internal/cli/status.go` - Add log reading and Claude analysis
- `internal/container/container.go` - Add LogWriter field to RunOptions, write to it if provided

## Non-Goals

- Real-time streaming to terminal (non-interactive mode)
- Detailed quality gate breakdown (keep it concise)
- Caching analysis results
