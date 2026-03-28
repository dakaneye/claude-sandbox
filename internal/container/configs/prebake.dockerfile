# syntax=docker/dockerfile:1
ARG BASE_IMAGE=claude-sandbox:base-arm64
FROM ${BASE_IMAGE}

SHELL ["/bin/bash", "-c"]

# Install Claude Code and prpm globally
RUN npm install -g @anthropic-ai/claude-code prpm && \
    claude --version && \
    prpm --version

# Fix known npm vulnerabilities in transitive dependencies
RUN npm audit fix 2>/dev/null || true && \
    npm update tar picomatch 2>/dev/null || true

# Pre-install the review-code skill for code quality gates
# prpm installs to flat directory structure: dakaneye-review-code (not @dakaneye/dakaneye-review-code)
# Use /home/claude since the container runs as user 'claude' (not root)
WORKDIR /home/claude
RUN prpm install @dakaneye/dakaneye-review-code --as claude && \
    test -d .claude/skills/dakaneye-review-code

# Install superpowers plugin for brainstorming, writing-plans, TDD, etc.
# Clone from public repo and write installed_plugins.json manually because
# `claude plugin install` requires marketplace auth not available at build time.
ARG SUPERPOWERS_VERSION=5.0.6
RUN mkdir -p .claude/plugins/cache/claude-plugins-official/superpowers && \
    git clone --depth 1 https://github.com/anthropics/claude-plugins-official.git /tmp/plugins && \
    cp -r /tmp/plugins/superpowers .claude/plugins/cache/claude-plugins-official/superpowers/${SUPERPOWERS_VERSION} && \
    rm -rf /tmp/plugins && \
    cat > .claude/plugins/installed_plugins.json << PLUGEOF
{
  "version": 2,
  "plugins": {
    "superpowers@claude-plugins-official": [
      {
        "scope": "user",
        "installPath": "/home/claude/.claude/plugins/cache/claude-plugins-official/superpowers/${SUPERPOWERS_VERSION}",
        "version": "${SUPERPOWERS_VERSION}",
        "installedAt": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)",
        "lastUpdated": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"
      }
    ]
  }
}
PLUGEOF

# Pre-configure Claude Code to skip onboarding and trust prompts
# Claude reads these from ~/.claude.json (not ~/.claude/settings.json)
# See: https://github.com/anthropics/claude-code/issues/4714
# See: https://github.com/anthropics/claude-code/issues/5572
RUN cat > .claude.json << 'EOF'
{
  "hasCompletedOnboarding": true,
  "hasTrustDialogAccepted": true,
  "hasTrustDialogHooksAccepted": true
}
EOF

# Permissions and other settings go in ~/.claude/settings.json
RUN cat > .claude/settings.json << 'EOF'
{
  "theme": "dark",
  "autoUpdaterStatus": "disabled",
  "permissions": {
    "allow": ["Bash", "Edit", "Write", "MultiEdit", "NotebookEdit", "Read", "Glob", "Grep", "WebFetch", "WebSearch"]
  },
  "plugins": {
    "superpowers@claude-plugins-official": true
  }
}
EOF

# Create global CLAUDE.md that references the pre-installed skill
# This makes /review-code available to all sandboxed Claude sessions
RUN cat > .claude/CLAUDE.md << 'EOF'
# Claude Sandbox Environment

## Pre-installed Skills

The review-code skill is pre-installed for code quality gates.

<!-- PRPM_MANIFEST_START -->

<skills_system priority="1">
<usage>
Skills provide specialized capabilities. Use the Skill tool to invoke them.
</usage>

<available_skills>

<skill>
<name>review-code</name>
<description>Comprehensive code review with language-specific expertise. Use PROACTIVELY after writing code, when reviewing PRs, or for security audits.</description>
<path>/home/claude/.claude/skills/dakaneye-review-code/SKILL.md</path>
</skill>

</available_skills>
</skills_system>

<!-- PRPM_MANIFEST_END -->
EOF
