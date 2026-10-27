# Contributing to RunPilot

Thank you for your interest in contributing to RunPilot!

## Core Principles

1. **Native Linux Process First**: We prioritize lightweight, native system processes over mandatory container virtualization.
2. **Strictly Node.js and Python Focused**: All runtimes and developer tooling are tailored for Node.js and Python ecosystems.
3. **Zero-PHP Invariant**: RunPilot must never introduce PHP or Composer checks, dependencies, or scripts into the core installer, control plane, or agent.
4. **Security by Default**: All actions performed by the host agent must be authenticated, strictly typed, argument-validated, and unprivileged.

## Development Setup

1. Requirements:
   - Go 1.22+
   - Node.js 18+ (for frontend testing)
   - Python 3.10+ (for Python runtime testing)
   - Git, Make

2. Build all binaries:
   ```bash
   make build
   ```

3. Run test suites:
   ```bash
   make test
   make test-gate
   ```

## Pull Request Guidelines

- Ensure all automated unit and security tests pass.
- Write unit tests for all new API endpoints and runtime handlers.
- Document changes in `docs/`.
