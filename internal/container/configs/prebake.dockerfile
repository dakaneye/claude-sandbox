# syntax=docker/dockerfile:1
ARG BASE_IMAGE=claude-sandbox:base-arm64
FROM ${BASE_IMAGE}

SHELL ["/bin/bash", "-c"]

# Switch to root to patch system-level packages and install global npm packages
USER root

# Patch vulnerable npm bundled dependencies before any npm operations
# picomatch 4.0.3 -> 4.0.4 (GHSA-c2c7-rcm5-vvqj, GHSA-3v7f-55p6-f55p)
# brace-expansion 5.0.4 -> 5.0.5 (GHSA-f886-m6hf-6m8v)
RUN cd /tmp && \
    npm pack picomatch@4.0.4 2>/dev/null && \
    tar xzf picomatch-4.0.4.tgz -C /usr/lib/node_modules/npm/node_modules/tinyglobby/node_modules/picomatch --strip-components=1 && \
    npm pack brace-expansion@5.0.5 2>/dev/null && \
    tar xzf brace-expansion-5.0.5.tgz -C /usr/lib/node_modules/npm/node_modules/brace-expansion --strip-components=1 && \
    rm -f /tmp/*.tgz

# Switch back to non-root user for all subsequent operations
USER claude

# Install Claude Code and prpm globally, then force-update tar
# tar 6.2.1 -> latest (GHSA-34x7-hfp2-rc4v and 5 others via prpm dep)
RUN npm install -g @anthropic-ai/claude-code prpm && \
    npm install -g tar@latest && \
    claude --version && \
    prpm --version

# Pre-install the review-code skill for code quality gates
# prpm installs to flat directory structure: dakaneye-review-code (not @dakaneye/dakaneye-review-code)
# Use /home/claude since the container runs as user 'claude' (not root)
WORKDIR /home/claude
RUN prpm install @dakaneye/dakaneye-review-code --as claude && \
    test -d .claude/skills/dakaneye-review-code

# Add default plugin marketplaces (skipped by hasCompletedOnboarding=true)
# and install superpowers plugin for brainstorming, writing-plans, TDD, etc.
RUN claude plugin marketplace add anthropics/claude-plugins-official && \
    claude plugin marketplace add anthropics/claude-code --sparse .claude-plugin plugins && \
    claude plugin install superpowers@claude-plugins-official

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
