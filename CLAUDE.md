# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Install dependencies
make deps

# Generate embedded files (processes //go:embed directives)
make generate

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

PicoClaw is an ultra-lightweight AI assistant written in Go, designed to run on resource-constrained hardware with minimal resource footprint. The architecture follows a message-bus pattern with pluggable channels and tools.

### Core Components

```
cmd/picoclaw/main.go          # CLI entry point, command routing
pkg/
├── agent/
│   ├── loop.go               # Main agent loop, LLM iteration, tool execution
│   ├── context.go            # System prompt builder, skill/memory loading
│   ├── context_window.go     # Context window budget tracking and allocation
│   ├── bootstrap.go          # Bootstrap file truncation for identity/context files
│   ├── pruning.go            # Context pruning with message summarization
│   ├── multipart_summary.go  # Large content summarization in chunks
│   ├── tool_truncation.go    # Tool result truncation with head/tail preservation
│   └── memory.go             # Long-term memory store
├── auth/
│   ├── oauth.go              # OAuth authentication flows (for Codex, etc.)
│   ├── pkce.go               # PKCE implementation for OAuth
│   └── store.go              # Token storage and management
├── bus/
│   ├── bus.go                # Message bus for inbound/outbound messages
│   └── types.go              # Message types (InboundMessage, OutboundMessage)
├── channels/
│   ├── manager.go            # Channel manager (start/stop all channels)
│   ├── base.go               # Base channel interface and utilities
│   └── discord.go            # Discord bot implementation (only channel)
├── config/
│   └── config.go             # Configuration loading with environment variable support
├── cron/
│   └── service.go            # Scheduled job execution
├── devices/
│   ├── service.go            # Device monitoring service
│   ├── events/               # Event source interfaces
│   └── sources/              # Platform-specific event sources (USB on Linux)
├── health/
│   └── server.go             # Health check HTTP server (/health, /ready)
├── heartbeat/
│   └── service.go            # Periodic task execution (HEARTBEAT.md)
├── logger/
│   └── logger.go             # Structured JSON logging
├── markdown/
│   └── frontmatter.go        # Markdown frontmatter parsing with YAML support
├── providers/
│   ├── types.go              # LLMProvider interface, Message/ToolCall types
│   └── [providers]           # http_provider.go (handles: openai, groq, openrouter, zhipu, vllm, gemini, nvidia, ollama, moonshot, shengsuanyun, deepseek), codex_provider.go, codex_cli_provider.go, github_copilot_provider.go
├── session/
│   └── manager.go            # Session history and summarization
├── skills/
│   └── loader.go             # Skill loading from workspace/global/builtin directories
├── state/
│   └── state.go              # Atomic state persistence
├── tools/
│   ├── registry.go           # Tool registration and execution
│   ├── base.go               # Tool interfaces (Tool, ContextualTool, AsyncTool)
│   ├── toolloop.go           # Reusable LLM+tool iteration loop (used by main agent and subagents)
│   ├── filesystem.go         # read_file, write_file, list_dir, edit_file, append_file
│   ├── shell.go              # exec (command execution with sandboxing)
│   ├── web.go                # web_search, web_fetch
│   ├── spawn.go              # spawn (async subagent creation)
│   ├── subagent.go           # subagent (sync subagent execution)
│   ├── message.go            # message (send messages to channels)
│   ├── cron.go               # cron tool for scheduling
│   ├── memory.go             # memory_read, memory_write, memory_append (per-user memory)
│   └── [hardware]            # i2c.go, spi.go (Linux-only)
├── reload/
│   ├── watcher.go            # File watching with fsnotify for hot reload
│   ├── manager.go            # Reload orchestration for config/skills/bootstrap
│   └── signals.go            # SIGHUP signal handling for manual reload
├── utils/
│   ├── media.go              # Media processing utilities
│   ├── message.go            # Message splitting utilities (shared across channels)
│   └── string.go             # String manipulation utilities
└── voice/
    └── transcriber.go        # Groq Whisper transcription for Discord voice messages
```

### Message Flow

1. **Inbound**: Channel → MessageBus.Inbound → AgentLoop
2. **Processing**: AgentLoop builds context → calls LLM → executes tools
3. **Outbound**: AgentLoop → MessageBus.Outbound → Channel → User

### Key Architectural Patterns

**Tool Registry**: All tools implement the `Tool` interface with `Name()`, `Description()`, `Parameters()`, and `Execute()`. Tools are registered in a `ToolRegistry` and converted to provider-compatible schemas.

**ToolLoop**: The `pkg/tools/toolloop.go` contains `RunToolLoop()`, which is the core reusable agent logic that handles LLM iteration and tool execution. This function is used by both the main agent loop and subagents, ensuring consistent behavior across all agent instances.

**Async Tools**: Tools can implement `AsyncTool` to return immediately and notify completion via callback. This is used for `spawn` (creates independent subagent) and long-running operations.

**Contextual Tools**: Tools can implement `ContextualTool` to receive channel/chatID context, enabling the `message` tool to send responses to the correct destination.

**Subagents**: The `spawn` tool creates async subagents with independent contexts; the `subagent` tool creates synchronous subagents. Both use the same provider and workspace but have separate tool registries (no spawn/subagent tools to prevent recursion).

**Context Building**: The `ContextBuilder` in `pkg/agent/context.go` constructs the system prompt by loading components in order: IDENTITY.md → SOUL.md → AGENTS.md → USER.md → Skills (workspace > global > builtin) → USER.md (again) → Memory → Tools summary. This ensures proper layering of identity, behavior, and user preferences.

**Hot Reload Architecture**: The reload package provides dynamic configuration updates:
- **File watcher** (`pkg/reload/watcher.go`): Uses fsnotify to watch config, bootstrap files (IDENTITY.md, SOUL.md, AGENTS.md, USER.md), and skills directories with 300ms debouncing
- **Reload manager** (`pkg/reload/manager.go`): Orchestrates safe updates to agent loop, context builder, and tool registry
- **Signal handler** (`pkg/reload/signals.go`): Catches SIGHUP for manual reload trigger
- **Agent reload methods**: `UpdateModel()`, `UpdateContextWindow()`, `UpdateBootstrapConfig()`, `UpdatePruningConfig()`, `ReloadTools()`, `InvalidateBootstrapCache()`, `ReloadSkillsSummary()`
- **Context cache**: `ContextBuilder` has bootstrap cache with `InvalidateBootstrapCache()` and `ReloadSkillsSummary()` methods
- **Config comparison**: `Config.CompareHotReloadable()` detects changed fields for selective reloading

**Skills**: Skills are markdown files (SKILL.md) in workspace skills directories. They provide additional instructions/context to the LLM. Skills load in priority: workspace > global (~/.picoclaw/skills) > builtin (./skills in repo).

**Session Management**: Conversations are stored in `workspace/sessions/`. History is automatically summarized when exceeding 75% of context window or 20 messages.

**Context Window Management**: The agent intelligently manages context window budgets with:
- **Bootstrap truncation**: Identity/context files (AGENTS.md, SOUL.md, IDENTITY.md, USER.md) are truncated with head/tail preservation (70%/20% ratio) when exceeding per-file (20K chars) or total (24K chars) limits
- **Budget tracking**: ContextWindow tracks token allocation across bootstrap content, message history, and tool results
- **Pruning**: Old messages are pruned and summarized using multipart summarization when approaching context limits
- **Tool truncation**: Large tool results are truncated with head/tail preservation to prevent context overflow

**Security Sandbox**: When `restrict_to_workspace` is true, file/command tools are restricted to the workspace directory. The `exec` tool additionally blocks dangerous commands (rm -rf, format, dd, shutdown, fork bombs) and supports configurable command filtering lists.

**Slash Commands**: The agent supports built-in slash commands:
- `/show model` - Display current model
- `/show channel` - Display current channel
- `/help` - Show available commands

**Discord Features**:
- **Message splitting**: Long messages are automatically split at 1500 characters (Discord limit is 2000) with code block preservation
- **Reply-to modes**: Configurable reply behavior - `first` (only first chunk replies), `all` (all chunks reply), or `off` (no replies)
- **Smart splitting**: Preserves code blocks by detecting unclosed ``` and extending buffer

**Hot Reload** (gateway mode only): PicoClaw supports hot reload of configuration, skills, and bootstrap files without restart:
- **Config reload**: Changes to `~/.picoclaw/config.json` automatically reload model, max_tokens, temperature, tool configs
- **Skills reload**: Adding/removing skills from `workspace/skills/` is detected automatically
- **Bootstrap reload**: Changes to IDENTITY.md, SOUL.md, AGENTS.md, USER.md invalidate the bootstrap cache
- **SIGHUP support**: Send `kill -HUP <pid>` to manually trigger a config reload
- **Debouncing**: 300ms debounce prevents duplicate reload events
- **Implementation**: Uses `fsnotify` for file watching; reload manager handles safe component updates

## Features Stripped Down from Main Branch

This branch is a minimal version of PicoClaw with ~8,077 lines of code removed (net -3,479 lines) from the main branch. The following features were removed or simplified:

### Channels Removed
- **Removed channels**: Telegram, Slack, WhatsApp, Line, DingTalk, Feishu (32-bit and 64-bit), QQ, OneBot, MaixCam
- **Remaining**: Discord only
- **Files removed**:
  - `pkg/channels/telegram.go` (491 lines)
  - `pkg/channels/slack.go` (404 lines + 174 lines tests)
  - `pkg/channels/whatsapp.go` (184 lines)
  - `pkg/channels/line.go` (598 lines)
  - `pkg/channels/dingtalk.go` (193 lines)
  - `pkg/channels/feishu_32.go` (36 lines)
  - `pkg/channels/feishu_64.go` (218 lines)
  - `pkg/channels/qq.go` (243 lines)
  - `pkg/channels/onebot.go` (686 lines)
  - `pkg/channels/maixcam.go` (243 lines)
  - `pkg/channels/manager.go` - simplified version (117 lines removed, channel manager logic simplified)

### Providers Removed
- **Removed providers**: Claude (direct), Claude CLI
- **Remaining**: Codex, Codex CLI, GitHub Copilot, HTTP provider (handles OpenAI, Groq, OpenRouter, etc.)
- **Files removed**:
  - `pkg/providers/claude_provider.go` (207 lines + 210 lines tests)
  - `pkg/providers/claude_cli_provider.go` (221 lines + 126 lines integration tests + 981 lines tests)
  - `pkg/providers/anthropic/` - entire directory removed

### Migration Package Removed
- **Removed entirely**: `pkg/migrate/` package
- **Files removed**:
  - `pkg/migrate/config.go` (382 lines)
  - `pkg/migrate/migrate.go` (394 lines + 854 lines tests)
  - `pkg/migrate/workspace.go` (106 lines)

### Agent Features Added
While stripping down features, this branch adds new agent capabilities:
- **Added**: `pkg/agent/bootstrap.go` (153 lines) - Bootstrap file truncation
- **Added**: `pkg/agent/context_window.go` (72 lines) - Context budget tracking
- **Added**: `pkg/agent/pruning.go` (207 lines) - Context pruning with message summarization
- **Added**: `pkg/agent/multipart_summary.go` (265 lines) - Large content summarization
- **Added**: `pkg/agent/tool_truncation.go` (94 lines) - Tool result truncation

### Hot Reload Added
- **Added**: `pkg/reload/` package (new feature not in main)
- **Files added**:
  - `pkg/reload/watcher.go` (348 lines + 348 lines tests) - File watching with fsnotify
  - `pkg/reload/manager.go` (278 lines + 262 lines tests) - Reload orchestration
  - `pkg/reload/signals.go` (85 lines) - SIGHUP signal handling

### Recent Updates from Main Branch (Cherry-picked)
- **Model fallback chain**: Automatic model fallback with error classification and cooldown
- **Cron execution timeout**: Configurable timeout for cron job execution
- **SKILL.md frontmatter**: Improved regex parsing for skill metadata
- **Build optimizations**: Optimized LDFLAGS for smaller binary size
- **CI improvements**: golangci-lint configuration and GitHub Actions enhancements
- **Documentation**: Added Vietnamese and Brazilian Portuguese READMEs
- **Dependencies**: Logic to prevent frequent dependency updates

### Summary Table
| Category | Main Branch | This Branch | Change |
|----------|-------------|-------------|--------|
| **Channels** | 10+ (Telegram, Slack, WhatsApp, Line, DingTalk, Feishu, QQ, OneBot, MaixCam, Discord) | 1 (Discord only) | -9 channels |
| **Providers** | Claude, Claude CLI, Codex, Codex CLI, GitHub Copilot, HTTP | Codex, Codex CLI, GitHub Copilot, HTTP | -2 providers |
| **Migration** | Full migrate package | Removed | -382-854 lines |
| **Agent Features** | Basic | Bootstrap truncation, context window, pruning, tool truncation | +791 lines |
| **Hot Reload** | None | File watcher, reload manager, SIGHUP | +1,323 lines |
| **Net Code Change** | - | - | -3,479 lines |

### Design Philosophy
This stripped-down branch prioritizes:
1. **Single-channel focus** - Discord only, reducing maintenance burden
2. **Core LLM providers** - Keep widely-used providers (OpenRouter, OpenAI, Groq, etc. via HTTP)
3. **Enhanced agent features** - Added advanced context management for better efficiency
4. **Hot reload capability** - Added for better development/operation experience
5. **Minimal dependencies** - Removed migration tools and channel-specific complexity

This makes the branch suitable for:
- Discord-focused deployments
- Resource-constrained environments
- Users who only need one messaging platform
- Development and testing of agent features

For full multi-channel support, consider using the main branch.

## Configuration

Config file location: `~/.picoclaw/config.json`

Key configuration sections:
- `agents.defaults`: Model, workspace, additional_memory_dir, max_tokens, temperature, max_tool_iterations, bootstrap_max_chars, bootstrap_total_max_chars, context_pruning (mode, ttl_minutes, keep_last_assistants, soft_trim_ratio, hard_clear_ratio, min_prunable_tool_chars)
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
├── memory/            # Long-term memory (MEMORY.md) - shared for CLI mode
│   └── users/         # Per-user memory for Discord users
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

### Additional Memory Directory

The `agents.defaults.additional_memory_dir` configuration allows merging an external `MEMORY.md` file with the primary workspace memory. This is useful for:

- Sharing system-wide knowledge across multiple workspaces
- Maintaining a centralized knowledge base
- Separating project-specific memory from shared reference material
- Providing Discord users with shared knowledge in addition to their personal memory

**Configuration:**
```json
{
  "agents": {
    "defaults": {
      "additional_memory_dir": "/path/to/shared/memory"
    }
  }
}
```

**Path resolution:**
- **Absolute paths**: Used as-is (e.g., `/var/lib/picoclaw-memory`)
- **Relative paths**: Resolved against workspace (e.g., `../shared-memory` → `workspace/../shared-memory`)
- **Tilde expansion**: `~` expands to user's home directory (e.g., `~/picoclaw-memory`)

**Behavior:**
- **All modes**: Both CLI and Discord users receive the additional shared memory merged with their primary memory
- **CLI mode**: Reads from `workspace/memory/MEMORY.md` + additional memory
- **Discord mode**: Reads from `workspace/memory/users/<USER_ID>/MEMORY.md` + additional memory
- **Graceful degradation**: If the additional directory doesn't exist, the agent continues with primary memory only
- **Hot reload supported**: Changes to `additional_memory_dir` are detected and tools are recreated automatically
- **No duplication**: If the additional path resolves to the same location as primary memory, it's not loaded twice
- **Read-only**: The additional memory is only merged when reading. Write/append operations only affect primary workspace memory (shared for CLI, per-user for Discord)

**Environment variable:**
```bash
export PICOCLAW_AGENTS_DEFAULTS_ADDITIONAL_MEMORY_DIR="/path/to/shared/memory"
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
- **Token estimation**: Uses rune count * 2 / 5 for CJK-aware estimation (2.5 chars per token) with 20% safety margin.
- **Platform-specific code**: Use build tags (e.g., `i2c_linux.go`, `i2c_other.go`) for platform-specific implementations.
- **Health check endpoints**: The gateway exposes `/health` (liveness) and `/ready` (readiness) endpoints at `http://host:port/health` and `/ready` for container orchestration probes. Configure host and port via `gateway.host` and `gateway.port` in config.
- **Voice transcription**: Discord voice messages are automatically transcribed using Groq's Whisper API when Groq is configured. The transcription happens in the Discord channel before the text is sent to the agent.
- **Device monitoring**: On Linux, the devices service can monitor USB hotplug events when `devices.monitor_usb` is enabled in config. Events are published to the message bus and can trigger agent actions.
- **Structured logging**: The logger package outputs structured JSON logs with configurable log levels (DEBUG, INFO, WARN, ERROR, FATAL). All logs include timestamp, level, component, and optional fields.
- **AGENTS.md symlink**: The root `AGENTS.md` is symlinked to `CLAUDE.md`, which means agent behavior guidance is loaded from the same file that guides Claude Code development.
- **Per-User Memory**: Discord users have isolated memory directories at `workspace/memory/users/<USER_ID>/`, while CLI mode uses shared `workspace/memory/`. The memory tools (`memory_read`, `memory_write`, `memory_append`) automatically route to the correct location based on the current user context. All users can additionally receive shared knowledge from `additional_memory_dir` when configured (read-only merge on read operations).

## Embedded Workspace

The binary contains embedded default workspace files via Go's `//go:embed` directive in `cmd/picoclaw/main.go`:

```go
//go:generate cp -r ../../workspace .
//go:embed workspace
var embeddedFiles embed.FS
```

When the binary first runs, it extracts these embedded files to the user's workspace directory (`~/.picoclaw/workspace`). This ensures every installation has the default AGENTS.md, IDENTITY.md, SOUL.md, and other essential configuration files. The `make generate` command processes the `//go:generate` directive to copy the workspace files before building.

### Embedded Workspace Templates

The embedded workspace includes default templates:
- **AGENTS.md**: Agent behavior guide with frontmatter metadata
- **TOOLS.md**: Local tool notes template
- **HEARTBEAT.md**: Periodic task template
- **IDENTITY.md**: Agent identity template
- **SOUL.md**: Agent soul template
- **USER.md**: User preferences template
