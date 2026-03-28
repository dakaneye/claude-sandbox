#!/usr/bin/env bash
set -Eeuo pipefail

# E2E workflow test: spec -> execute -> status -> clean
# Requires: claude-sandbox binary in PATH, ANTHROPIC_API_KEY set, Docker running
#
# This test costs API credits and takes 5-10 minutes. Run manually or in CI,
# not on every commit.
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

# Ensure we're in a git repo
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "=== E2E Workflow Test ==="
echo "Session: $SESSION_NAME"
echo ""

# Pre-flight checks
command -v claude-sandbox >/dev/null 2>&1 || { echo "Error: claude-sandbox not in PATH" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Error: docker not found" >&2; exit 1; }
[[ -n "${ANTHROPIC_API_KEY:-}" ]] || { echo "Error: ANTHROPIC_API_KEY not set" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Error: docker daemon not running" >&2; exit 1; }

# 1. Create a session with a simple PLAN.md
echo "--- Step 1: Creating session with PLAN.md ---"
# spec runs interactively, so we create the session and write PLAN.md directly
claude-sandbox spec --name "$SESSION_NAME" </dev/null || true

SESSION_DIR="$REPO_ROOT/.claude-sandbox/sessions"
WORKTREE_PATH=$(jq -r .worktree_path "$SESSION_DIR/$SESSION_NAME.json" 2>/dev/null)
if [[ -z "$WORKTREE_PATH" || "$WORKTREE_PATH" == "null" ]]; then
    echo "Error: could not find session worktree path" >&2
    exit 1
fi
cat > "$WORKTREE_PATH/PLAN.md" << 'PLAN'
# Test Plan

Create a file `hello.txt` containing "Hello World".
Commit the file.
Write COMPLETION.md with Status: SUCCESS.
PLAN
echo "PLAN.md written to $WORKTREE_PATH"

# 2. Execute
echo ""
echo "--- Step 2: Execute ---"
timeout 600 claude-sandbox execute --session "$SESSION_NAME"
EXECUTE_EXIT=$?
if [[ $EXECUTE_EXIT -ne 0 ]]; then
    echo "Warning: execute exited with code $EXECUTE_EXIT"
fi

# 3. Status (should show completed state, not crash)
echo ""
echo "--- Step 3: Status ---"
STATUS_OUTPUT=$(claude-sandbox status --session "$SESSION_NAME" 2>&1)
echo "$STATUS_OUTPUT"

if echo "$STATUS_OUTPUT" | grep -qE "(completed successfully|Failed|Blocked)"; then
    echo "✓ Status command works for completed session"
else
    echo "✗ Status output unexpected" >&2
    exit 1
fi

# 4. Clean
echo ""
echo "--- Step 4: Clean ---"
claude-sandbox clean --session "$SESSION_NAME" --all
echo "✓ Clean completed"
