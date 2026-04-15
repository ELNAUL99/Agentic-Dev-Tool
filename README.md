# Go-Agent

AI-assisted backend code agent for generating, testing, reviewing, and tracking Go backend work from feature descriptions.

`go-agent` is a command-line tool. It does not currently include a browser UI. The main interface is the `go-agent` CLI.

## Overview

Go-Agent turns a feature request, such as `Add restaurant search endpoint`, into an agent workflow:

1. Plan the work as ordered tasks.
2. Generate Go code and tests.
3. Run tests locally or in a Docker sandbox.
4. Attempt automatic fixes for failing tests.
5. Review generated code with a reviewer agent.
6. Save workflow state and optionally use Git integration.
7. Store workflow memory for future context.

The project is useful as a prototype or development assistant for backend code generation experiments. Generated code should still be reviewed by a human before production use.

## Features

- CLI workflow for feature generation.
- Planner agent for task breakdown.
- Coder agent for Go implementation and test generation.
- Reviewer agent that scores code quality and returns issues.
- OpenAI-compatible LLM client.
- Local and Docker-based test execution.
- Auto-fix loop for failing generated tests.
- Plugin system with built-in helper plugins.
- Memory store for past workflow decisions.
- Git branch, add, and commit integration.
- JSON workflow state saved on disk.
- Example generated restaurant search package.

## Project Status

This project is an experimental CLI-first developer tool. It is suitable for local exploration, testing agent workflows, and extending the architecture. It is not yet a polished production platform and does not include a web dashboard.

## Requirements

- Go 1.23 or newer.
- Docker, if using the default sandboxed test runner.
- An OpenAI API key or an OpenAI-compatible API endpoint.
- Git, if using Git integration.
- Make, optional but convenient for build/test commands.

## Installation

Build the binary from the project root:

```bash
cd /Users/macbook/Desktop/coding/go-agent
make build
```

The binary is created at:

```bash
./build/go-agent
```

You can also run it without building:

```bash
go run ./cmd/go-agent help
```

To install it into your Go binary path:

```bash
make install
```

## Quick Start

Initialize local configuration:

```bash
./build/go-agent init
```

Set your API key:

```bash
export OPENAI_API_KEY="sk-..."
```

Run an interactive feature workflow:

```bash
./build/go-agent create-feature "Add restaurant search endpoint" -i
```

Check workflow status:

```bash
./build/go-agent status <WORKFLOW_ID>
```

Run review or tests for a saved workflow:

```bash
./build/go-agent review <WORKFLOW_ID>
./build/go-agent test <WORKFLOW_ID>
```

## Command Reference

| Command | Description |
| --- | --- |
| `init` | Create `.go-agent/config.json` with default settings. |
| `create-feature [TITLE]` | Run the full planning, generation, testing, review, and summary workflow. |
| `status [WORKFLOW_ID]` | Show saved workflow state, tasks, and generated artifacts. |
| `review [WORKFLOW_ID]` | Run code review for an existing workflow. |
| `test [WORKFLOW_ID]` | Run tests for artifacts in an existing workflow. |
| `generate-tests [WORKFLOW_ID]` | Generate test files for existing workflow artifacts. |
| `approve [WORKFLOW_ID]` | Mark a workflow approved and optionally commit when auto-commit is enabled. |
| `reject [WORKFLOW_ID]` | Mark a workflow rejected. |
| `help` | Show CLI help. |

### Create Feature Options

```bash
./build/go-agent create-feature "Add payment gateway" \
  --context ./docs/api-spec.yaml \
  -t backend,payments \
  -i
```

Supported flags:

- `-i`, `--interactive`: run with interactive prompts.
- `--context FILES`: comma-separated files to include as project context.
- `-t`, `--tags TAGS`: comma-separated workflow tags.

## Configuration

Configuration lives at:

```bash
.go-agent/config.json
```

Create it with:

```bash
./build/go-agent init
```

Example configuration:

```json
{
  "llm": {
    "provider": "openai",
    "model": "gpt-4o",
    "api_key": "",
    "base_url": "",
    "temperature": 0.2,
    "max_tokens": 4096,
    "timeout_seconds": 120
  },
  "agents": {
    "planner": { "enabled": true, "max_retries": 3 },
    "coder": { "enabled": true, "max_retries": 3 },
    "reviewer": { "enabled": true, "max_retries": 2 }
  },
  "plugins": {
    "directory": ".go-agent/plugins",
    "enabled": ["db-schema", "api-validator"],
    "config": {}
  },
  "git": {
    "enabled": true,
    "auto_commit": false,
    "branch_prefix": "agent/",
    "commit_prefix": "[agent] ",
    "remote_name": "origin"
  },
  "docker": {
    "enabled": true,
    "image": "golang:1.23-alpine",
    "timeout_seconds": 300,
    "memory_limit": "512m",
    "cpu_limit": "1.0",
    "network_mode": "none"
  },
  "memory": {
    "enabled": true,
    "store_path": ".go-agent/memory.db",
    "max_items": 1000
  },
  "workflows": {
    "defaults_path": "configs/workflows"
  }
}
```

### Environment Variables

These environment variables override config values:

- `OPENAI_API_KEY`: API key used by the LLM client.
- `OPENAI_BASE_URL`: optional OpenAI-compatible base URL.
- `OPENAI_MODEL`: model name override.

## Workflow Lifecycle

### 1. Planning

The planner agent receives the feature spec and returns ordered development tasks. Tasks include IDs, titles, descriptions, task types, dependencies, priority, and expected files.

### 2. Code Generation

The coder agent generates artifacts in a structured format. Artifacts are written to their requested file paths and tracked in workflow state.

### 3. Test Generation And Execution

The coder agent can generate tests for Go artifacts. The test runner then executes tests either locally or in Docker.

### 4. Auto-Fix

When tests fail, the orchestrator asks the coder agent to fix the relevant artifacts. The workflow makes a limited number of fix attempts.

### 5. Code Review

The reviewer agent evaluates generated code for correctness, Go idioms, error handling, security, performance, testability, and documentation.

### 6. Git And Summary

When Git integration is enabled, the orchestrator can create a branch, stage generated artifacts, and commit changes. It also prints a PR-style summary.

### 7. Memory

The memory store records workflow summaries so future workflows can retrieve relevant context.

## Architecture

```text
CLI
  |
  v
Orchestrator
  |
  +--> Planner Agent
  |
  +--> Plugin Manager
  |
  +--> Coder Agent
  |
  +--> Test Runner
  |
  +--> Reviewer Agent
  |
  +--> Git Integration
  |
  +--> Memory Store
```

Key packages:

- `cmd/go-agent`: CLI entry point.
- `internal/cli`: command parsing and user-facing CLI output.
- `internal/orchestrator`: workflow state machine.
- `internal/agents/planner`: feature-to-task planning.
- `internal/agents/coder`: code, test, fix, and refine generation.
- `internal/agents/reviewer`: generated code review.
- `pkg/llm`: OpenAI-compatible HTTP client.
- `internal/testrunner`: local and Docker test execution.
- `internal/plugins`: built-in plugin manager and plugins.
- `internal/memory`: JSON-backed memory store.
- `internal/git`: Git command integration.
- `internal/models`: shared workflow, task, artifact, review, and test types.
- `internal/config`: application configuration.

## Generated Files And State

Go-Agent writes local project state under:

```bash
.go-agent/
```

Common paths:

- `.go-agent/config.json`: local configuration.
- `.go-agent/workflows/<WORKFLOW_ID>.json`: workflow state.
- `.go-agent/memory.db`: JSON memory store.
- `.go-agent/plugins/`: intended location for custom plugins.

Generated code artifacts are written to the file paths returned by the coder agent. The sample generated package in this repository lives under:

```bash
examples/generated/
```

## Testing

Run the project test suite:

```bash
go test ./...
```

Run tests with verbose output and coverage:

```bash
make test
```

Run static checks:

```bash
go vet ./...
```

If your environment cannot write to the default Go build cache, use a writable cache path:

```bash
GOCACHE=/private/tmp/go-agent-go-build-cache go vet ./...
```

## Docker Sandbox

Docker test execution is enabled by default. It runs generated artifacts in an isolated container with:

- `--network none`
- memory limit from config, default `512m`
- CPU limit from config, default `1.0`
- image from config, default `golang:1.23-alpine`

Disable Docker test execution by setting:

```json
{
  "docker": {
    "enabled": false
  }
}
```

When Docker is disabled, tests run locally with `go test`.

## Plugin System

Built-in plugins:

- `db-schema`: suggests database schema structure for model and database tasks.
- `api-validator`: suggests API design concerns such as validation, auth, and pagination.
- `load-test`: suggests benchmark and load-test scenarios.

Enabled plugins are configured in `.go-agent/config.json`:

```json
{
  "plugins": {
    "enabled": ["db-schema", "api-validator", "load-test"]
  }
}
```

Per-plugin config can disable a listed plugin:

```json
{
  "plugins": {
    "enabled": ["db-schema"],
    "config": {
      "db-schema": {
        "enabled": false
      }
    }
  }
}
```

## Memory System

The memory system stores workflow summaries and tags in `.go-agent/memory.db`. It can retrieve relevant entries by content, context, tags, and workflow type.

Memory is enabled by default:

```json
{
  "memory": {
    "enabled": true,
    "store_path": ".go-agent/memory.db",
    "max_items": 1000
  }
}
```

## Git Integration

Git integration can create a workflow branch, stage generated artifacts, and commit them. Auto-commit is disabled by default.

Default Git config:

```json
{
  "git": {
    "enabled": true,
    "auto_commit": false,
    "branch_prefix": "agent/",
    "commit_prefix": "[agent] ",
    "remote_name": "origin"
  }
}
```

This repository snapshot may not itself be inside a Git repository. Git commands require running the tool from a Git worktree.

## Development

Useful Make targets:

```bash
make build       # Build ./build/go-agent
make test        # Run go test -v -cover ./...
make install     # Install the CLI with go install
make clean       # Remove build output and workflow state
make dev-run ARGS="help"
```

Run the CLI during development:

```bash
go run ./cmd/go-agent help
go run ./cmd/go-agent init
go run ./cmd/go-agent create-feature "Add search endpoint" -i
```

## Project Structure

```text
go-agent/
├── cmd/go-agent/              # CLI entry point
├── configs/workflows/         # Example workflow configuration
├── examples/generated/        # Sample generated Go backend code
├── internal/
│   ├── agents/
│   │   ├── coder/             # Code, test, fix, and refine agent
│   │   ├── planner/           # Planning agent
│   │   └── reviewer/          # Review agent
│   ├── cli/                   # CLI commands and output
│   ├── codegen/               # Code artifact generator helpers
│   ├── config/                # Config loading and defaults
│   ├── git/                   # Git integration
│   ├── memory/                # Memory persistence and retrieval
│   ├── models/                # Shared types
│   ├── orchestrator/          # Workflow engine
│   ├── plugins/               # Built-in plugin system
│   └── testrunner/            # Local and Docker test runners
├── pkg/llm/                   # OpenAI-compatible LLM client
├── Dockerfile                 # Container image definition
├── Makefile                   # Development commands
├── go.mod                     # Go module
└── README.md                  # Project documentation
```

## Example Generated Package

The repository includes a sample generated restaurant package:

```bash
examples/generated/
```

It contains:

- `restaurant.go`: domain types and search filters.
- `handler.go`: HTTP handler methods.
- `restaurant_test.go`: tests for generated behavior.

This sample package is not a standalone web app. It is example backend code that can be imported or wired into an HTTP server.

## Security Notes

- Do not commit API keys or secrets.
- Prefer `OPENAI_API_KEY` over hard-coding credentials in config.
- Docker sandboxing helps isolate generated test execution, but generated code still requires review.
- The default Docker network mode is `none`.
- Git integration shells out to `git`; use it only in repositories where generated changes are expected.
- LLM-generated code can contain incorrect assumptions, vulnerable patterns, or incomplete edge-case handling.

## Troubleshooting

### `OPENAI_API_KEY` is missing

Set the environment variable:

```bash
export OPENAI_API_KEY="sk-..."
```

### Docker is unavailable

Either start Docker or disable Docker in `.go-agent/config.json`:

```json
{
  "docker": {
    "enabled": false
  }
}
```

### Git branch or commit fails

Make sure you are running inside a Git repository. You can also disable Git integration:

```json
{
  "git": {
    "enabled": false
  }
}
```

### Config file is missing

Run:

```bash
./build/go-agent init
```

### Go cache permission errors

Use a writable cache directory:

```bash
GOCACHE=/private/tmp/go-agent-go-build-cache go test ./...
```

### No browser interface appears

This is expected. Go-Agent currently exposes a CLI interface only.

## Limitations

- No web dashboard or browser UI.
- Requires LLM access for planning, generation, review, and fixes.
- Generated code should be reviewed before production use.
- External plugin loading is scaffolded but not fully implemented.
- Git features require running inside a Git repository.
- Docker sandboxing depends on Docker availability.
- Memory retrieval is simple keyword and tag scoring, not vector search.

## Roadmap

Potential future improvements:

- Web UI for workflow visualization.
- Richer workflow state browser.
- GitHub or GitLab PR creation.
- More LLM provider presets.
- Vector-based semantic memory.
- External plugin loading.
- CI integration for generated workflows.
- Better artifact diffing before writes.

## Contributing

Contributions should keep the project simple, testable, and CLI-first unless a web UI is intentionally added.

Recommended workflow:

1. Run `go test ./...`.
2. Run `go vet ./...`.
3. Add focused tests for behavior changes.
4. Keep generated artifacts and local `.go-agent/` state out of commits unless they are intentional examples.

## License

No license file is currently included in this repository. Add a `LICENSE` file before distributing or reusing the project outside private/local development.
