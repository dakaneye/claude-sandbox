# Security Policy

## Reporting Vulnerabilities

**Preferred**: [GitHub Security Advisories](https://github.com/dakaneye/claude-sandbox/security/advisories/new) - allows private discussion and coordinated disclosure.

**Alternative**: Email the repository owner directly (see GitHub profile).

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if you have one)

## Response Timeline

This is a personal project maintained in spare time. Realistic expectations:

- **Acknowledgment**: Within 7 days
- **Initial assessment**: Within 14 days
- **Fix for critical issues**: As soon as possible, typically within 30 days

For critical vulnerabilities affecting active users, I'll prioritize accordingly.

## Scope

### In Scope

**Container Security**
- Container escape vulnerabilities
- Privilege escalation within containers
- Insecure default configurations
- Volume mount security issues

**Credential Handling**
- API key exposure (Anthropic, GitHub tokens)
- Credential leakage in logs or error messages
- Insecure credential storage or transmission

**Supply Chain**
- Dependency vulnerabilities in Go modules
- Container base image security (apko-built images)
- Build pipeline security (GitHub Actions)

**Code Execution**
- Command injection via CLI arguments
- Path traversal in worktree operations
- Unsafe handling of git operations

### Out of Scope

- Vulnerabilities in Claude itself or Anthropic's API
- Issues requiring physical access to the host machine
- Social engineering attacks
- Denial of service through resource exhaustion (containers already have limits)
- Security issues in Docker itself

## Security Model

claude-sandbox provides defense-in-depth for running AI coding assistants:

1. **Container isolation**: Workloads run in Docker containers with limited capabilities
2. **Worktree isolation**: Each session operates on an isolated git worktree
3. **Credential injection**: API keys passed via environment variables, not persisted in containers
4. **No network by default**: Containers can be run with restricted network access

This tool reduces risk but does not eliminate it. Running any AI coding assistant involves inherent risks from generated code execution. Users should review changes before merging to main branches.

## Supported Versions

Only the latest release receives security updates. This project does not maintain LTS branches.
