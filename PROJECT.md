# Go-Agent: Project Architecture & Design

## Overview

**Go-Agent** is a production-grade, AI-assisted backend code generation tool written entirely in Go with **zero external dependencies**. It demonstrates agentic development workflows through a multi-agent system that plans, codes, tests, reviews, and integrates code into Git repositories.

---

## Core Design Philosophy

### 1. Zero External Dependencies
The entire project uses only Go's standard library. This makes it:
- **Auditable**: Every line of code is reviewable without navigating dependency trees
- **Portable**: Builds on any Go installation without network access for `go mod download`
- **Stable**: No risk of dependency deprecation, CVEs, or breaking changes

### 2. Human-in-the-Loop
The system is explicitly designed so that **humans remain in control** at every critical decision point. The agent handles repetitive, mechanical work while humans review, approve, or override strategic decisions.

### 3. Sandbox-First Execution
All test execution runs inside Docker by default with:
- `--network none` (no outbound connections)
- `--memory 512m` (resource limits)
- `--cpus 1.0` (CPU limits)

This ensures generated code cannot harm the host system.

---

## What Does the Agent Own vs. What Does a Human Review?

| Phase | Agent Owns | Human Reviews/Approves |
|-------|-----------|----------------------|
| **Planning** | Breaks spec into ordered tasks with dependencies | Approves/rejects the plan before code generation begins |
| **Code Generation** | Writes Go structs, interfaces, handlers, services | Reviews generated files, requests refinements |
| **Test Generation** | Creates table-driven tests with mocks and benchmarks | Reviews test coverage and edge cases |
| **Test Execution** | Runs `go test`, parses failures, attempts auto-fixes | Decides if auto-fixed code is acceptable |
| **Code Review** | Scores code 0-100, identifies security/performance issues | Reviews score, approves or requests improvements |
| **Git Integration** | Creates branches, stages files, commits | Approves final PR, merges (human owns merge) |
| **Memory/Learning** | Stores decisions and outcomes for future context | None (fully automated background process) |

**Key Principle**: The agent never commits to `main`, never pushes without approval, and never deletes existing code. All changes live on `agent/*` branches until explicitly approved.

---

## Multi-Agent System

### Planner Agent (`internal/agents/planner`)
**Role**: Converts natural language specs into structured development tasks.

**Prompt Strategy**: Uses a system prompt that instructs the LLM to output JSON arrays of tasks with proper dependency ordering. Includes rules like "infrastructure first, features second" and "include test tasks for every code task."

**Human Checkpoint**: After planning, the human sees the task list and approves/rejects before any code is generated.

### Coder Agent (`internal/agents/coder`)
**Role**: Generates Go source files and tests from approved tasks.

**Capabilities**:
- Generates idiomatic Go with proper error handling, context propagation, and interface design
- Creates table-driven tests with mock-friendly interfaces
- Auto-fixes failing tests (up to 3 iterations) by sending test output back to the LLM
- Refines code based on human feedback

**Human Checkpoint**: After code generation, human can review files and request refinements before tests run.

### Reviewer Agent (`internal/agents/reviewer`)
**Role**: Evaluates code quality across 7 dimensions.

**Review Dimensions**:
1. Correctness — does it work?
2. Go idioms — does it feel like Go?
3. Error handling — are errors wrapped and propagated?
4. Security — injection risks, race conditions, leaks
5. Performance — obvious bottlenecks
6. Testing — is it testable? Are edge cases covered?
7. Documentation — are exported items documented?

**Approval Criteria**: Score ≥ 80 AND zero critical issues. Otherwise, the human sees the review and decides whether to approve or request fixes.

---

## Workflow Engine (Orchestrator)

The orchestrator (`internal/orchestrator`) implements a 5-phase pipeline:

```
Spec → [Planner] → Tasks → [Coder] → Code → [TestRunner] → Results → [Reviewer] → Review → [Git] → PR
          ↑                    ↑                ↑                      ↑
    Human approves       Human approves    Auto-fix loop (3x)     Human approves
```

**State Management**: Each workflow is persisted as JSON in `.go-agent/workflows/`. This allows:
- Resuming interrupted workflows
- Reviewing past decisions
- Debugging agent behavior

---

## Plugin System

Plugins extend the agent's capabilities without modifying core code.

**Built-in Plugins**:
- **db-schema**: Analyzes task descriptions and suggests database tables/columns
- **api-validator**: Checks REST design patterns (pagination, validation, auth)
- **load-test**: Recommends benchmark and load test strategies

**Plugin API**:
```go
type Plugin interface {
    Name() string
    Description() string
    Execute(ctx context.Context, task models.Task, config map[string]interface{}) (map[string]string, error)
}
```

Plugins run during the code generation phase and their output is injected into the coder agent's prompt as additional context.

---

## Memory System

The memory system (`internal/memory`) stores workflow outcomes for continuous improvement.

**What Gets Stored**:
- Feature descriptions and generated task structures
- Code patterns that worked well
- Human feedback and override decisions

**Retrieval**: Before generating a new feature, the system queries past workflows by tag and content similarity. Relevant memories are injected into the prompt as "Project conventions" context.

**Storage**: Simple JSON file (`.go-agent/memory.db`) with configurable max items. No vector DB required, though the architecture supports upgrading to one.

---

## Test Runner

**Dual Execution Modes**:

### Local Mode
Runs `go test -v -cover ./...` directly. Suitable for trusted environments or CI pipelines.

### Docker Sandbox (Default)
Creates a temporary directory, writes all artifacts, then executes:
```bash
docker run --rm \
  --network none \
  --memory 512m \
  --cpus 1.0 \
  -v /tmp/go-agent-test-xxx:/workspace \
  -w /workspace \
  golang:1.23-alpine \
  sh -c "go mod tidy && go test -v -cover ./..."
```

**Auto-Fix Loop**: If tests fail, the test output is sent back to the Coder Agent with a "fix this" prompt. This loop repeats up to 3 times.

---

## Git Integration

**Branch Strategy**:
- All agent work happens on `agent/{workflow-id}` branches
- Prefix is configurable (`branch_prefix: "agent/"`)
- Default base branch is the current branch (usually `main`)

**Commit Strategy**:
- Commits use prefix `[agent]` by default
- `auto_commit: false` means human must explicitly run `go-agent approve`
- Files are staged individually, not with `git add .`

**Safety**: The agent never force-pushes, never deletes branches, and never modifies protected branches.

---

## LLM Client Design

The LLM client (`pkg/llm`) implements an interface over OpenAI-compatible APIs:

```go
type Client interface {
    Complete(ctx context.Context, messages []models.PromptMessage, opts *CompletionOptions) (*models.LLMResponse, error)
    CompleteStream(ctx context.Context, messages []models.PromptMessage, opts *CompletionOptions, handler StreamHandler) error
}
```

**Features**:
- Supports any OpenAI-compatible endpoint (local models, proxies, etc.)
- JSON mode via `response_format: {type: "json_object"}`
- Streaming support for real-time output
- Configurable temperature, max tokens, timeout
- Custom headers for authentication (e.g., Azure OpenAI)

---

## Configuration

Configuration is JSON-based (not YAML) to maintain the zero-dependency constraint. The `go-agent init` command generates a complete default configuration.

**Environment Overrides** (take precedence over config file):
- `OPENAI_API_KEY` — LLM API key
- `OPENAI_BASE_URL` — API endpoint
- `OPENAI_MODEL` — Model selection

---

## Testing Strategy

The project includes comprehensive unit tests for all non-LLM components:
- **config**: Default values, loading, merging
- **models**: Type validation, state transitions
- **memory**: Store/retrieve, relevance scoring, persistence
- **plugins**: Plugin execution, relevance detection
- **testrunner**: Failure parsing, coverage extraction

Tests for LLM-dependent components (planner, coder, reviewer) are omitted because they require API keys and non-deterministic outputs. These are best tested through integration tests in CI.

---

## Example Output

See `examples/generated/` for sample agent output:
- `restaurant.go` — Domain model, service, and repository interface
- `restaurant_test.go` — Table-driven tests with mock repository
- `handler.go` — HTTP handlers with query parameter parsing

See `examples/workflow-state.json` for a persisted workflow showing the full state the agent tracks.

---

## Future Extensibility

The architecture supports these upgrades without rewrites:
1. **New LLM providers**: Implement `llm.Client` interface
2. **New agent roles**: Add to `AgentsConfig` and orchestrator
3. **New plugins**: Implement `Plugin` interface and register with manager
4. **Vector memory**: Replace `memory.Store` with Milvus/Pinecone backend
5. **Web UI**: Add HTTP server that wraps orchestrator with REST API
6. **CI/CD integration**: GitHub Actions that call `go-agent create-feature` from issue labels

---

## Security Considerations

1. **Prompt Injection**: User input (feature specs) is sent to the LLM. The system does not execute LLM output directly — it only writes files and runs `go test` in a sandbox.
2. **Secret Leakage**: The LLM prompt does not include `.env` files, API keys, or secrets. Context files must be explicitly provided via `--context`.
3. **Code Execution**: Generated code is never executed on the host. It only runs inside Docker with no network access.
4. **Git Safety**: The agent cannot push to protected branches or force-push.
