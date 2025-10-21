# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**agentenv** is a CLI tool for running multiple isolated development environments from git worktrees. It enables running multiple LLM coding agents (Claude, Codex, etc.) simultaneously with isolated Docker services, databases, and configurations.

Key concepts:
- Each agent gets a **git worktree** (separate working directory)
- Each agent gets **unique ports** via port slot allocation (slot 1 = base+1, slot 2 = base+2, etc.)
- Docker services are isolated using **override files** and renamed volumes/containers
- Environment files are **patched** with agent-specific values (ports, database names, paths)
- A **registry** (`/.agentenv/registry.json`) tracks all active agents

## Common Commands

### Build and Install
```bash
make build      # Build the agentenv binary
make install    # Build and install to /usr/local/bin (requires sudo)
make test       # Run tests
go fmt ./...    # Format code
go vet ./...    # Run go vet
```

### Testing the Tool
```bash
# Must be run from a project directory with .agentenv.yml
./agentenv up <agent-name> <branch> <command>
./agentenv list
./agentenv down <agent-id>
./agentenv version
```

## Architecture

### Core Workflow (cmd/up.go)

The `agentenv up` command follows this sequence:
1. Load `.agentenv.yml` config (internal/config)
2. Load registry (internal/registry)
3. Find next available port slot (gap-filling: reuses slots from removed agents)
4. Allocate ports for all services (basePort + slot)
5. Create git worktree (internal/git)
6. Generate docker-compose override file (internal/docker)
7. Patch environment files with agent-specific values (internal/envpatch)
8. Run pre-start setup commands
9. Start Docker services
10. Run post-start setup commands
11. Save registry
12. Launch agent in terminal (internal/terminal)

### Port Allocation

Ports use a **slot-based system** where slot numbers are reused:
- Registry tracks `PortSlot` (1, 2, 3, ...) and `Ports` (actual allocated ports)
- `FindNextAvailableSlot()` finds the lowest unused slot (gaps are filled)
- Each service gets: `basePort + slot` (e.g., postgres:5432 + slot 2 = 5434)
- This allows removing agent1 and creating agent5, which will reuse slot 1

### Registry Structure

Located at `.agentenv/registry.json`:
- Tracks active agents by ID (agent ID = agent name now)
- Each agent has: name, branch, ports, portSlot, worktreePath, createdAt, etc.
- Registry must be saved after allocation/removal

### Docker Override Generation (internal/docker/compose.go)

The override file provides:
- Unique container names: `{project}_{service}_agent{slot}`
- Port mappings: `{allocatedPort}:{containerPort}`
- Renamed volumes: `{volumeName}_agent{slot}`
- Templated environment variables: `{id}`, `{service.port}`, `{worktree_path}`

**Important**: Base docker-compose.yml should NOT define port mappings. Ports are only defined in override files to prevent conflicts. For local development, create a `docker-compose.override.yml` (add to .gitignore).

### Environment Patching (internal/envpatch)

Uses regex patterns from config to replace values in env files:
- Supports templates: `{postgres.port}`, `{backend.port}`, `{id}`, `{worktree_path}`
- Example: `postgresql://user:pass@localhost:5432/db` → `postgresql://user:pass@localhost:5433/db_agent1`

### Git Worktrees (internal/git/worktree.go)

- Worktrees are created as sibling directories: `{repoDir}-{agentName}`
- Branch is created if it doesn't exist
- Removal uses `git worktree remove --force` by default

### Package Organization

```
cmd/              # Cobra CLI commands (up, down, list, export)
internal/
  config/         # Config loading and parsing (.agentenv.yml)
  registry/       # Agent registry management
  docker/         # Docker compose override generation
  git/            # Git worktree operations
  envpatch/       # Environment file patching
  env/            # Legacy env patcher (deprecated?)
  database/       # Database export functionality
  terminal/       # Terminal launcher for agents
```

## Go Style Guide (STYLE_GUIDE.md)

This project follows strict coding principles:

### Abstraction Rules
- **Rule 1.1**: Default to a single function first
- **Rule 1.2**: Justify every abstraction (don't create helpers/types prematurely)
- **Rule 3.1**: Rule of Three - only refactor on third occurrence of duplication

### Function Rules
- **Rule 2.1**: Functions do one thing (describable in one sentence)
- **Rule 2.2**: Max 50 lines per function
- **Rule 2.3**: Max 4 parameters (use structs or methods on structs instead)
- **Rule 2.4**: Return max 2 values directly, use named structs for 3+ values

### Package and Interface Rules
- **Rule 4.1**: Packages have singular purpose (no "utils", "common", "helpers")
- **Rule 4.2**: Interfaces defined by consumer, not producer
- **Rule 4.3**: Keep interfaces small (ideally 1 method, max 3)

**When writing code**: Always check if you can stay in a single function first. Only create helper functions when hitting function length limits (50 lines) or parameter limits (4 params).

## Dependencies

- **github.com/spf13/cobra**: CLI framework
- **gopkg.in/yaml.v3**: YAML parsing for config and docker-compose files
- **github.com/lib/pq**: PostgreSQL driver (for database export)

## Configuration File (.agentenv.yml)

The tool expects a `.agentenv.yml` in the project root. Key sections:
- `docker.services`: Service definitions with ports, volumes, environment
- `env_files`: Files to patch with regex patterns
- `setup_commands`: Commands to run before/after services start
- `agent_launch`: Terminal configuration for launching agents
- `cleanup`: Database archival and volume removal settings

## Notes for Development

- Agent IDs are now just the agent name (e.g., "claude1"), not generated IDs
- The `NextID` field in registry is deprecated but kept for backward compatibility
- Port slots are gap-filled - removing agent1 frees slot 1 for the next agent
- Base docker-compose.yml is modified in worktrees (ports removed) to avoid conflicts
- Environment patching happens before Docker services start
- Setup commands can run before or after services start (controlled by `when` field)
