#!/usr/bin/env bash
# sandbox-guard.sh - Block external side effects in sandbox mode
# Exit 0 = allow, Exit 2 = block
set -euo pipefail

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
TOOL_INPUT=$(echo "$INPUT" | jq -r '.tool_input // empty')

# ============================================
# MCP Tool Gates
# ============================================
if [[ "$TOOL_NAME" == mcp__* ]]; then
    # Linear MCP - block writes
    BLOCKED_MCP=(
        "mcp__linear__createIssue"
        "mcp__linear__updateIssue"
        "mcp__linear__createComment"
        "mcp__linear__addLabel"
        "mcp__linear__removeLabel"
        "mcp__linear__archiveIssue"
    )

    for blocked in "${BLOCKED_MCP[@]}"; do
        if [[ "$TOOL_NAME" == "$blocked" ]]; then
            echo "BLOCKED: MCP write operation not allowed in sandbox" >&2
            echo "  Tool: $TOOL_NAME" >&2
            echo "  Use 'claude-sandbox ship' after review to perform external actions." >&2
            exit 2
        fi
    done

    # Allow all other MCP (reads)
    exit 0
fi

# ============================================
# Bash Command Gates
# ============================================
if [[ "$TOOL_NAME" == "Bash" ]]; then
    CMD=$(echo "$TOOL_INPUT" | jq -r '.command // empty')

    # --- Always blocked (catastrophic) ---
    CATASTROPHIC=(
        'rm -rf /'
        'rm -rf ~'
        'rm -rf \$HOME'
        'rm -rf /Users'
        'rm -rf /home'
        'chmod -R 777 /'
        'chown -R.*/'
    )

    for pattern in "${CATASTROPHIC[@]}"; do
        if echo "$CMD" | grep -qiE "$pattern"; then
            echo "BLOCKED: Catastrophic command" >&2
            exit 2
        fi
    done

    # --- Git push (any form) ---
    if echo "$CMD" | grep -qE '\bgit\s+push\b'; then
        echo "BLOCKED: git push not allowed in sandbox" >&2
        echo "  Use 'claude-sandbox ship' after review." >&2
        exit 2
    fi

    # --- GitHub CLI writes ---
    GH_WRITE_PATTERNS=(
        'gh\s+pr\s+create'
        'gh\s+pr\s+merge'
        'gh\s+pr\s+close'
        'gh\s+pr\s+comment'
        'gh\s+pr\s+review'
        'gh\s+issue\s+create'
        'gh\s+issue\s+close'
        'gh\s+issue\s+comment'
        'gh\s+release\s+create'
    )

    for pattern in "${GH_WRITE_PATTERNS[@]}"; do
        if echo "$CMD" | grep -qE "$pattern"; then
            echo "BLOCKED: gh write operation not allowed in sandbox" >&2
            echo "  Command: $CMD" >&2
            echo "  Use 'claude-sandbox ship' after review." >&2
            exit 2
        fi
    done

    # --- HTTP write methods ---
    HTTP_WRITE_PATTERNS=(
        'curl.*-X\s*POST'
        'curl.*-X\s*PUT'
        'curl.*-X\s*PATCH'
        'curl.*-X\s*DELETE'
        'curl.*--data'
        'curl.*-d\s'
        'curl.*--json'
        'wget.*--post-data'
        'wget.*--post-file'
    )

    for pattern in "${HTTP_WRITE_PATTERNS[@]}"; do
        if echo "$CMD" | grep -qiE "$pattern"; then
            echo "BLOCKED: HTTP write operation not allowed in sandbox" >&2
            echo "  Command: $CMD" >&2
            exit 2
        fi
    done

    # Allow everything else
    exit 0
fi

# ============================================
# All other tools - allow
# ============================================
exit 0
