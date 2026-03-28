#!/usr/bin/env bash
set -Eeuo pipefail

# E2E functional test: spec -> status (running) -> status (completed) -> clean
#
# Tests the full CLI workflow without execute (which takes 10+ minutes due to
# hardcoded quality gates). The execute step is simulated by writing a log file
# and updating session state, then status is verified against real parsing.
#
# For the full workflow including execute, run: test/e2e/full_workflow_test.sh
#
# ship is excluded -- creates real PRs, would pollute repos with test garbage

SESSION_NAME="e2e-test-$$"
REPO_ROOT=""

cleanup() {
    local exit_code=$?
    echo ""
    if [[ -n "$REPO_ROOT" ]]; then
        echo "Cleaning up..."
        claude-sandbox clean --session "$SESSION_NAME" 2>/dev/null || true
    fi
    if [[ $exit_code -eq 0 ]]; then
        echo "✓ E2E workflow passed"
    else
        echo "✗ E2E workflow failed (exit code: $exit_code)"
    fi
    exit $exit_code
}
trap cleanup EXIT

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "=== E2E Functional Test ==="
echo "Session: $SESSION_NAME"
echo ""

# Pre-flight
command -v claude-sandbox >/dev/null 2>&1 || { echo "Error: claude-sandbox not in PATH" >&2; exit 1; }

# --- Step 1: Create session via spec ---
echo "--- Step 1: Create session ---"
claude-sandbox spec --name "$SESSION_NAME" </dev/null || true

SESSION_DIR="$REPO_ROOT/.claude-sandbox/sessions"
SESSION_FILE="$SESSION_DIR/$SESSION_NAME.json"
if [[ ! -f "$SESSION_FILE" ]]; then
    echo "Error: session file not created" >&2
    exit 1
fi

WORKTREE_PATH=$(jq -r .worktree_path "$SESSION_FILE")
echo "Session created, worktree: $WORKTREE_PATH"

# Write PLAN.md so session is in ready state
cat > "$WORKTREE_PATH/PLAN.md" << 'PLAN'
# Test Plan
Create hello.txt containing "Hello World".
PLAN

# --- Step 2: Simulate a running session with a real log file ---
echo ""
echo "--- Step 2: Status during running session ---"

# Write a realistic log file
LOG_DIR="$HOME/.claude/sandbox-sessions"
mkdir -p "$LOG_DIR"
LOG_PATH="$LOG_DIR/$(jq -r .id "$SESSION_FILE").log"

cat > "$LOG_PATH" << 'LOG'
{"type":"system","subtype":"init","session_id":"test-session"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cat /workspace/PLAN.md","description":"Read plan"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"# Test Plan\nCreate hello.txt"}]},"timestamp":"2026-03-28T03:31:38.037Z"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"make build 2>&1","description":"Run build"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"BUILD OK"}]},"timestamp":"2026-03-28T03:32:00.000Z"}
{"type":"system","subtype":"task_progress","description":"Running build"}
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./...","description":"Run tests"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","content":"ok"}]},"timestamp":"2026-03-28T03:33:00.000Z"}
{"type":"system","subtype":"task_progress","description":"Running tests"}
LOG

# Update session to running state with log path
STARTED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
jq --arg status "running" \
   --arg started "$STARTED_AT" \
   --arg log "$LOG_PATH" \
   '.status = $status | .started_at = $started | .log_path = $log' \
   "$SESSION_FILE" > "$SESSION_FILE.tmp" && mv "$SESSION_FILE.tmp" "$SESSION_FILE"

# Run status -- THIS IS THE MAIN BUG FIX ASSERTION: must not crash
STATUS_OUTPUT=$(claude-sandbox status --session "$SESSION_NAME" 2>&1)
STATUS_EXIT=$?
echo "$STATUS_OUTPUT"

if [[ $STATUS_EXIT -ne 0 ]]; then
    echo "✗ Status command crashed (exit $STATUS_EXIT)" >&2
    exit 1
fi
echo "✓ Status did not crash for running session"

# Verify it shows progress info (either haiku analysis or fallback)
if echo "$STATUS_OUTPUT" | grep -qE "(tool calls|Analysis unavailable|Phase|Execution in progress|%)"; then
    echo "✓ Status shows progress information"
else
    echo "✗ Status missing progress info" >&2
    echo "  Output was: $STATUS_OUTPUT" >&2
    exit 1
fi

# --- Step 3: Simulate completed session ---
echo ""
echo "--- Step 3: Status after completion ---"

# Write COMPLETION.md
cat > "$WORKTREE_PATH/COMPLETION.md" << 'COMPLETION'
# Completion

## Status: SUCCESS

Created hello.txt as specified.
COMPLETION

# Update session to success state
COMPLETED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
jq --arg status "success" \
   --arg completed "$COMPLETED_AT" \
   '.status = $status | .completed_at = $completed | .error = ""' \
   "$SESSION_FILE" > "$SESSION_FILE.tmp" && mv "$SESSION_FILE.tmp" "$SESSION_FILE"

STATUS_OUTPUT=$(claude-sandbox status --session "$SESSION_NAME" 2>&1)
echo "$STATUS_OUTPUT"

if echo "$STATUS_OUTPUT" | grep -q "completed successfully"; then
    echo "✓ Status shows completion"
else
    echo "✗ Status missing completion message" >&2
    exit 1
fi

# --- Step 4: Ship (dry-run) ---
echo ""
echo "--- Step 4: Ship (dry-run) ---"
SHIP_OUTPUT=$(claude-sandbox ship --session "$SESSION_NAME" --dry-run 2>&1)
echo "$SHIP_OUTPUT"

if echo "$SHIP_OUTPUT" | grep -q "Dry run: would launch Claude"; then
    echo "✓ Ship dry-run validated"
else
    echo "✗ Ship dry-run unexpected output" >&2
    exit 1
fi

# --- Step 5: Clean ---
echo ""
echo "--- Step 5: Clean ---"
claude-sandbox clean --session "$SESSION_NAME" --all
echo "✓ Clean completed"
