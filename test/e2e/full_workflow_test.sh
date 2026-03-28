#!/usr/bin/env bash
set -Eeuo pipefail

# Full E2E workflow test: spec -> execute -> status -> clean
#
# WARNING: This test costs API credits and takes 10-20 minutes.
# The execute command runs Claude with full quality gates (build, lint, test,
# /review-code grade A) which requires a complete iteration cycle.
#
# For a fast functional test, run: test/e2e/workflow_test.sh
#
# ship is excluded -- creates real PRs, would pollute repos with test garbage

SESSION_NAME="e2e-full-$$"
REPO_ROOT=""

cleanup() {
    local exit_code=$?
    echo ""
    # Stop any running containers for this session
    docker ps -q --filter "name=claude-sandbox" | xargs -r docker stop 2>/dev/null || true
    if [[ -n "$REPO_ROOT" ]]; then
        echo "Cleaning up..."
        claude-sandbox clean --session "$SESSION_NAME" 2>/dev/null || true
    fi
    if [[ $exit_code -eq 0 ]]; then
        echo "✓ Full E2E workflow passed"
    else
        echo "✗ Full E2E workflow failed (exit code: $exit_code)"
    fi
    exit $exit_code
}
trap cleanup EXIT

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo "=== Full E2E Workflow Test ==="
echo "Session: $SESSION_NAME"
echo "Expected duration: 10-20 minutes"
echo ""

# Pre-flight
command -v claude-sandbox >/dev/null 2>&1 || { echo "Error: claude-sandbox not in PATH" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Error: docker not found" >&2; exit 1; }
[[ -n "${ANTHROPIC_API_KEY:-}" ]] || { echo "Error: ANTHROPIC_API_KEY not set" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "Error: docker daemon not running" >&2; exit 1; }

# 1. Create session
echo "--- Step 1: Create session ---"
claude-sandbox spec --name "$SESSION_NAME" </dev/null || true

SESSION_DIR="$REPO_ROOT/.claude-sandbox/sessions"
SESSION_FILE="$SESSION_DIR/$SESSION_NAME.json"
WORKTREE_PATH=$(jq -r .worktree_path "$SESSION_FILE")

cat > "$WORKTREE_PATH/PLAN.md" << 'PLAN'
# Test Plan

Create a file called `hello.txt` containing "Hello World".
Commit it, then write COMPLETION.md with Status: SUCCESS.
PLAN
echo "PLAN.md written to $WORKTREE_PATH"

# 2. Execute (20 minute timeout)
echo ""
echo "--- Step 2: Execute (up to 20 minutes) ---"
timeout 1200 claude-sandbox execute --session "$SESSION_NAME"
EXECUTE_EXIT=$?
if [[ $EXECUTE_EXIT -eq 124 ]]; then
    echo "✗ Execute timed out after 20 minutes" >&2
    exit 1
fi

# 3. Status
echo ""
echo "--- Step 3: Status ---"
STATUS_OUTPUT=$(claude-sandbox status --session "$SESSION_NAME" 2>&1)
echo "$STATUS_OUTPUT"

if echo "$STATUS_OUTPUT" | grep -qE "(completed successfully|Failed|Blocked)"; then
    echo "✓ Status shows terminal state"
else
    echo "✗ Status output unexpected" >&2
    exit 1
fi

# 4. Clean
echo ""
echo "--- Step 4: Clean ---"
claude-sandbox clean --session "$SESSION_NAME" --all
echo "✓ Clean completed"
