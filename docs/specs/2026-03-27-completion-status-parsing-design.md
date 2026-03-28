# COMPLETION.md Status Parsing Fix

## Problem

Both `execute` and `ship` commands detect session status by exact substring match:

```go
strings.Contains(string(content), "Status: SUCCESS")
```

Claude writes freeform markdown (e.g., `**Status**: Complete — Grade A`), which doesn't match. A successful session gets marked as failed, and ship refuses to create a PR.

## Solution

1. Update `buildClaudeCommand` prompt to request a machine-readable first line
2. Add a shared `parseCompletionStatus` function with strict + fuzzy matching
3. Use it in both `execute.go` and `ship.go`

## Prompt Change

In `buildClaudeCommand` (container.go), replace:

```
Update COMPLETION.md with final status.
```

With:

```
When done, write COMPLETION.md. The FIRST LINE must be exactly one of:
STATUS: SUCCESS
STATUS: BLOCKED
STATUS: FAILED
Follow with details about what was done.
```

## Shared Function

```go
// parseCompletionStatus determines session status from COMPLETION.md content.
func parseCompletionStatus(content string) state.Status
```

**Location:** `internal/cli/completion.go` (new file, single responsibility)

**Algorithm — two-pass detection:**

1. **Strict pass:** Scan lines for one starting with `status:` (case-insensitive, trimmed). Parse the word after the colon. Map `success`/`complete` → StatusSuccess, `blocked` → StatusBlocked, `failed` → StatusFailed.

2. **Fuzzy fallback:** If no strict match, scan the full content (lowercased) for keywords. Priority order: blocked > success/complete > failed. This catches freeform markdown like `**Status**: Complete — Grade A`.

3. **Default:** If nothing matches, return StatusFailed.

**Priority rationale:** A file mentioning both "blocked" and "success" (e.g., "blocked from reaching success") should be treated as blocked. Blocked is the strongest negative signal.

## Files Changed

| File | Change |
|------|--------|
| `internal/cli/completion.go` | New: `parseCompletionStatus` function |
| `internal/cli/completion_test.go` | New: table-driven tests |
| `internal/cli/execute.go:176-189` | Replace inline matching with `parseCompletionStatus` |
| `internal/cli/ship.go:69-70` | Replace inline matching with `parseCompletionStatus` |
| `internal/container/container.go` | Update prompt in `buildClaudeCommand` |
| `internal/container/container_test.go` | Update test asserting prompt content |

## Testing

| Test | Input | Expected |
|------|-------|----------|
| Strict SUCCESS | `STATUS: SUCCESS\n\nDetails...` | StatusSuccess |
| Strict BLOCKED | `STATUS: BLOCKED\n\nDetails...` | StatusBlocked |
| Strict FAILED | `STATUS: FAILED\n\nDetails...` | StatusFailed |
| Strict lowercase | `status: success` | StatusSuccess |
| Strict with whitespace | `  Status:  SUCCESS  ` | StatusSuccess |
| Fuzzy complete | `**Status**: Complete — Grade A` | StatusSuccess |
| Fuzzy success in body | `## Status: All gates passed successfully` | StatusSuccess |
| Fuzzy blocked | `Could not proceed, blocked by lint` | StatusBlocked |
| Blocked beats success | `Blocked from reaching success` | StatusBlocked |
| No match | `# Some random markdown` | StatusFailed |
| Empty | `` | StatusFailed |

## Success Criteria

1. The 759066 session log (Grade A, `**Status**: Complete`) would be detected as success
2. `ship` accepts the same COMPLETION.md that `execute` marks as success
3. Existing `Status: SUCCESS` format still works
