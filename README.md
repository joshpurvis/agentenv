# agentenv - Multi-Agent Development Environment Tool

**agentenv** is a CLI tool for running multiple isolated development environments from git worktrees. It enables running multiple LLM coding agents (Claude, Codex, etc.) simultaneously, each with their own:

- Git worktree and branch
- Docker services with unique ports
- Database instance
- Environment configuration

## Features

- **Zero Conflicts**: Isolated ports, volumes, and databases per agent
- **Generic & Reusable**: Works with any Docker Compose project via config file
- **Production-Like**: Full stack testing (frontend, backend, database)
- **Clean Teardown**: Archive databases, clean up resources
- **Simple CLI**: Fast, intuitive commands

## Installation

### From Source

```bash
git clone <your-repo-url>
cd agentenv
go build -o agentenv .
sudo mv agentenv /usr/local/bin/
```

Or use the Makefile:

```bash
make build      # Build the binary
make install    # Install to /usr/local/bin
```

## Quick Start

1. **Create a configuration file** `.agentenv.yml` in your project root:

```yaml
docker:
  compose_file: docker-compose.yml
  services:
    postgres:
      ports:
        - container: 5432
          host_base: 5432
      volumes:
        - postgres_data
      environment:
        POSTGRES_DB: "myapp_agent{id}"
    
    backend:
      ports:
        - container: 8000
          host_base: 8000
    
    frontend:
      ports:
        - container: 5173
          host_base: 5173

env_files:
  - path: backend/.env
    patches:
      - pattern: 'postgresql://([^:]+):([^@]+)@([^:]+):(\d+)/(\w+)'
        replace: 'postgresql://\1:\2@\3:{postgres.port}/\5_agent{id}'

database:
  service: postgres
  type: postgresql
  
cleanup:
  archive_database: true
  archive_location: .agentenv/archives
  remove_volumes: true
```

2. **Launch an agent environment**:

```bash
cd ~/projects/myapp
agentenv up claude1 feat/new-feature claude
```

This will:
- Create a git worktree at `../myapp-claude1`
- Allocate unique ports (e.g., 8001, 5174, 5433)
- Generate docker-compose override file
- Patch environment files with agent-specific values
- Start Docker services
- Run setup commands (migrations, etc.)

3. **List active agents**:

```bash
agentenv list
```

Output:
```
Active Agents (2):

ID       Branch              Command   Ports                               Path
──       ──────              ───────   ─────                               ────
agent1   feat/new-feature    claude    postgres:5433, backend:8001...     /home/user/projects/myapp-agent1
agent2   feat/refactor       codex     postgres:5434, backend:8002...     /home/user/projects/myapp-agent2

To stop an agent:
  agentenv down <agent-id>
```

4. **Stop and cleanup an agent**:

```bash
agentenv down agent1
```

This will:
- Archive the database to `.agentenv/archives/agent1-TIMESTAMP.sql`
- Stop Docker services
- Remove volumes
- Remove git worktree
- Update registry

## Commands

### `agentenv up <agent-name> <branch> <command>`

Launch a new agent environment.

**Arguments**:
- `agent-name`: Unique name for this agent (e.g., `claude1`)
- `branch`: Git branch to checkout (will be created if it doesn't exist)
- `command`: Command to run in the agent environment (e.g., `claude`)

**Example**:
```bash
agentenv up claude1 feat/fix-rendering claude
```

### `agentenv down <agent-id>`

Stop and cleanup an agent environment.

**Arguments**:
- `agent-id`: Agent ID to stop (e.g., `agent1`)

**Flags**:
- `--skip-archive`: Skip database archival
- `--keep-worktree`: Keep the git worktree

**Example**:
```bash
agentenv down agent1
agentenv down agent2 --skip-archive
```

### `agentenv list`

List all active agent environments.

**Example**:
```bash
agentenv list
```

### `agentenv version`

Print version information.

**Example**:
```bash
agentenv version
```

## Configuration

The `.agentenv.yml` configuration file defines how `agentenv` manages your project.

### Docker Services

Define which Docker services to manage and their port allocation:

```yaml
docker:
  compose_file: docker-compose.yml
  services:
    postgres:
      ports:
        - container: 5432
          host_base: 5432
      volumes:
        - postgres_data
      environment:
        POSTGRES_DB: "myapp_agent{id}"
```

**Port Allocation**: Host port = `host_base + agent_id`
- Agent 1: `host_base + 1` (e.g., 5433)
- Agent 2: `host_base + 2` (e.g., 5434)

### Environment File Patching

Define regex patterns to patch environment files:

```yaml
env_files:
  - path: backend/.env
    patches:
      - pattern: 'DATABASE_URL=postgresql://(.+):(\d+)/(\w+)'
        replace: 'DATABASE_URL=postgresql://\1:{postgres.port}/\3_agent{id}'
```

**Template Variables**:
- `{service.port}`: Allocated port for a service (e.g., `{postgres.port}`)
- `{id}`: Agent numeric ID
- `{worktree_path}`: Absolute path to worktree

### Setup Commands

Define commands to run after services start:

```yaml
setup_commands:
  - name: "Run migrations"
    command: "dbmate up"
    working_dir: "."
    when: after_services_start
```

### Cleanup Configuration

Configure cleanup behavior:

```yaml
cleanup:
  archive_database: true
  archive_location: .agentenv/archives
  remove_volumes: true
```

## How It Works

### Port Allocation

Each agent gets unique ports by adding the agent ID to the base port:

| Service   | Base Port | Agent 1 | Agent 2 | Agent 3 |
|-----------|-----------|---------|---------|---------|
| Backend   | 8000      | 8001    | 8002    | 8003    |
| Frontend  | 5173      | 5174    | 5175    | 5176    |
| Postgres  | 5432      | 5433    | 5434    | 5435    |

### Git Worktrees

Git worktrees are created as sibling directories:

```
projects/
├── myapp/                    # Main repo
├── myapp-agent1/             # Agent 1 worktree
└── myapp-agent2/             # Agent 2 worktree
```

### Registry

The `.agentenv/registry.json` file tracks active agents:

```json
{
  "project": "myapp",
  "config_version": "1.0",
  "next_id": 3,
  "agents": {
    "agent1": {
      "name": "claude1",
      "branch": "feat/new-feature",
      "worktree_path": "/home/user/projects/myapp-agent1",
      "ports": {
        "postgres": 5433,
        "backend": 8001,
        "frontend": 5174
      },
      "created_at": "2025-01-20T10:30:00Z"
    }
  }
}
```

**Note**: Add `.agentenv/registry.json` to `.gitignore`

## Development

### Building

```bash
make build
```

### Installing Locally

```bash
make install
```

### Testing

```bash
make test
```

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
