# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Install dependencies
make deps

# Build for current platform (Linux/x86_64, Linux/arm64, Linux/riscv64, Darwin/arm64)
make build

# Build for all platforms
make build-all

# Install to ~/.local/bin
make install

# Run tests
make test

# Run linting/vet
make vet

# Format code
make fmt

# Run the agent (CLI mode)
./build/picoclaw-linux-amd64 agent -m "Hello"

# Run the gateway (chat apps)
./build/picoclaw-linux-amd64 gateway

# Check status
./build/picoclaw-linux-amd64 status

# Manage scheduled tasks
./build/picoclaw-linux-amd64 cron list
./build/picoclaw-linux-amd64 cron add -n "task" -m "prompt" -e 60
./build/picoclaw-linux-amd64 cron remove <id>

# Manage skills
./build/picoclaw-linux-amd64 skills list
./build/picoclaw-linux-amd64 skills install <repo>
```

### Running a Single Test

```bash
go test -v ./pkg/<package> -run TestFunctionName
```

## Architecture Overview

PicoClaw is an ultra-lightweight AI assistant written in Go, designed to run on resource-constrained hardware (<10MB RAM). The architecture follows a message-bus pattern with pluggable channels and tools.

### Core Components

```
cmd/picoclaw/main.go          # CLI entry point, command routing
pkg/
├── agent/
│   ├── loop.go               # Main agent loop, LLM iteration, tool execution
│   ├── context.go            # System prompt builder, skill/memory loading
│   └── memory.go             # Long-term memory store
├── bus/
│   ├── bus.go                # Message bus for inbound/outbound messages
│   └── types.go              # Message types (InboundMessage, OutboundMessage)
├── channels/
│   ├── manager.go            # Channel manager (start/stop all channels)
│   ├── base.go               # Base channel interface and utilities
│   └── discord.go            # Discord bot implementation (only channel)
├── tools/
│   ├── registry.go           # Tool registration and execution
│   ├── base.go               # Tool interfaces (Tool, ContextualTool, AsyncTool)
│   ├── filesystem.go         # read_file, write_file, list_dir, edit_file, append_file
│   ├── shell.go              # exec (command execution with sandboxing)
│   ├── web.go                # web_search, web_fetch
│   ├── spawn.go              # spawn (async subagent creation)
│   ├── subagent.go           # subagent (sync subagent execution)
│   ├── message.go            # message (send messages to channels)
│   ├── cron.go               # cron tool for scheduling
│   └── [hardware]            # i2c.go, spi.go (Linux-only)
├── providers/
│   ├── types.go              # LLMProvider interface, Message/ToolCall types
│   └── [providers]           # http_provider.go (handles: openai, groq, openrouter, zhipu, vllm, gemini, nvidia, ollama, moonshot, shengsuanyun, deepseek), codex_provider.go, codex_cli_provider.go, github_copilot_provider.go
├── config/
│   └── config.go             # Configuration loading with environment variable support
├── session/
│   └── manager.go            # Session history and summarization
├── skills/
│   └── loader.go             # Skill loading from workspace/global/builtin directories
├── heartbeat/
│   └── service.go            # Periodic task execution (HEARTBEAT.md)
├── cron/
│   └── service.go            # Scheduled job execution
└── state/
    └── state.go              # Atomic state persistence
```

### Message Flow

1. **Inbound**: Channel → MessageBus.Inbound → AgentLoop
2. **Processing**: AgentLoop builds context → calls LLM → executes tools
3. **Outbound**: AgentLoop → MessageBus.Outbound → Channel → User

### Key Architectural Patterns

**Tool Registry**: All tools implement the `Tool` interface with `Name()`, `Description()`, `Parameters()`, and `Execute()`. Tools are registered in a `ToolRegistry` and converted to provider-compatible schemas.

**Async Tools**: Tools can implement `AsyncTool` to return immediately and notify completion via callback. This is used for `spawn` (creates independent subagent) and long-running operations.

**Contextual Tools**: Tools can implement `ContextualTool` to receive channel/chatID context, enabling the `message` tool to send responses to the correct destination.

**Subagents**: The `spawn` tool creates async subagents with independent contexts; the `subagent` tool creates synchronous subagents. Both use the same provider and workspace but have separate tool registries (no spawn/subagent tools to prevent recursion).

**Skills**: Skills are markdown files (SKILL.md) in workspace skills directories. They provide additional instructions/context to the LLM. Skills load in priority: workspace > global (~/.picoclaw/skills) > builtin (./skills in repo).

**Session Management**: Conversations are stored in `workspace/sessions/`. History is automatically summarized when exceeding 75% of context window or 20 messages.

**Security Sandbox**: When `restrict_to_workspace` is true, file/command tools are restricted to the workspace directory. The `exec` tool additionally blocks dangerous commands (rm -rf, format, dd, shutdown, fork bombs).

## Configuration

Config file location: `~/.picoclaw/config.json`

Key configuration sections:
- `agents.defaults`: Model, workspace, max_tokens, temperature, max_tool_iterations
- `channels`: Discord credentials and allow lists
- `providers`: API keys for OpenRouter, OpenAI, Gemini, Zhipu, Groq, VLLM, Nvidia, Ollama, Moonshot, ShengSuanYun, DeepSeek, GitHub Copilot, Codex
- `tools.web`: Brave and DuckDuckGo search configuration
- `heartbeat`: Periodic task interval (minutes)
- `devices`: USB monitoring, hardware events
- `gateway`: Host and port configuration for health check endpoints

Environment variables override JSON config using the pattern `PICOCLAW_<SECTION>_<KEY>` (e.g., `PICOCLAW_AGENTS_DEFAULTS_MODEL`).

## Workspace Layout

```
~/.picoclaw/workspace/
├── sessions/          # Conversation history (JSON files per session)
├── memory/            # Long-term memory (MEMORY.md)
├── state/             # Persistent state (last channel, etc.)
├── cron/              # Scheduled jobs database (jobs.json)
├── skills/            # User-installed skills (workspace-level)
├── AGENTS.md          # Agent behavior guide (loaded into system prompt)
├── HEARTBEAT.md       # Periodic task instructions
├── IDENTITY.md        # Agent identity
├── SOUL.md            # Agent soul
├── TOOLS.md           # Tool descriptions (optional override)
└── USER.md            # User preferences
```

## Creating New Tools

1. Implement the `Tool` interface in `pkg/tools/`:
```go
type MyTool struct {
    workspace string
    restrict  bool
}

func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Description() string { return "Does something" }
func (t *MyTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "arg": map[string]interface{}{
                "type": "string",
                "description": "Argument",
            },
        },
        "required": []string{"arg"},
    }
}
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
    // Return SuccessResult(), ErrorResult(), SilentResult(), or AsyncResult()
}
```

2. Register in `createToolRegistry()` in `pkg/agent/loop.go`

## Creating New Channels

1. Implement the channel interface in `pkg/channels/`:
```go
type MyChannel struct {
    baseChannel
    config Config
}

func (c *MyChannel) Start(ctx context.Context) error { ... }
func (c *MyChannel) Stop() error { ... }
```

2. Add config struct to `pkg/config/config.go`
3. Register in `NewManager()` in `pkg/channels/manager.go`
4. Add to `ChannelsConfig` struct

## Adding LLM Providers

1. Create provider file in `pkg/providers/` implementing `LLMProvider` interface
2. Add config to `ProvidersConfig` in `pkg/config/config.go`
3. Add to `CreateProvider()` factory function

## Important Design Details

- **Orphaned tool messages**: The agent loop removes leading "tool" role messages from history to prevent LLM errors when tool call IDs are missing after summarization.
- **Heartbeat independence**: Heartbeat tasks use `ProcessHeartbeat()` which doesn't load session history - each heartbeat execution is independent.
- **Message deduplication**: The agent checks if the `message` tool already sent a response to avoid duplicate messages to users.
- **Token estimation**: Uses rune count / 3 for CJK-aware estimation (more accurate than byte length).
- **Platform-specific code**: Use build tags (e.g., `i2c_linux.go`, `i2c_other.go`) for platform-specific implementations.
- **Health check endpoints**: The gateway exposes `/health` (liveness) and `/ready` (readiness) endpoints at `http://host:port/health` and `/ready` for container orchestration probes.
