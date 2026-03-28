# syntax=docker/dockerfile:1
ARG BASE_IMAGE=claude-sandbox:base-arm64
FROM ${BASE_IMAGE}

SHELL ["/bin/bash", "-c"]

# Install Claude Code and prpm globally
RUN npm install -g @anthropic-ai/claude-code prpm && \
    claude --version && \
    prpm --version

# Pre-install the review-code skill for code quality gates
WORKDIR /home/claude
RUN prpm install @dakaneye/dakaneye-review-code --as claude && \
    test -d .claude/skills/dakaneye-review-code

# Add default plugin marketplaces (skipped by hasCompletedOnboarding=true)
# and install superpowers plugin for brainstorming, writing-plans, TDD, etc.
RUN claude plugin marketplace add anthropics/claude-plugins-official && \
    claude plugin marketplace add anthropics/claude-code --sparse .claude-plugin plugins && \
    claude plugin install superpowers@claude-plugins-official

# Patch all known vulnerable npm dependencies
# Must run as root to write to /usr/lib/node_modules (system npm)
USER root
RUN cd /tmp && \
    npm pack picomatch@4.0.4 2>/dev/null && \
    tar xzf picomatch-4.0.4.tgz -C /usr/lib/node_modules/npm/node_modules/tinyglobby/node_modules/picomatch --strip-components=1 && \
    npm pack brace-expansion@5.0.5 2>/dev/null && \
    tar xzf brace-expansion-5.0.5.tgz -C /usr/lib/node_modules/npm/node_modules/brace-expansion --strip-components=1 && \
    npm pack tar@latest 2>/dev/null && \
    tar xzf tar-*.tgz -C /home/claude/.npm-global/lib/node_modules/prpm/node_modules/tar --strip-components=1 && \
    rm -f /tmp/*.tgz
USER claude

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
