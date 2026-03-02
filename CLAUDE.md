# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the binary
go build ./cmd/mseep

# Run the compiled binary
./mseep --help

# Run tests
go test ./...

# Run specific package tests
go test ./internal/config
go test ./internal/fuzzy

# Clean up dependencies
go mod tidy
```

### CLI Usage Examples
```bash
# Quick commands with fuzzy matching
./mseep enable "github"     # Enable a server by fuzzy search
./mseep disable "burp"      # Disable a server by fuzzy search
./mseep toggle "obsidian"   # Toggle a server by fuzzy search

# TUI mode
./mseep tui                 # Launch interactive TUI

# Status and health
./mseep status              # Show status of clients and servers
./mseep status --client claude --json
./mseep health              # Run health checks
./mseep health --fix        # Auto-disable failing servers

# Apply changes
./mseep apply               # Apply canonical to all detected clients
./mseep apply --client cursor --profile dev

# Profile management
./mseep profiles list
./mseep profiles create dev server1 server2
./mseep profiles save myprofile
./mseep profiles apply dev
```

## Architecture

This is a Go-based CLI/TUI tool for managing MCP (Model Context Protocol) servers across different AI clients (Claude Desktop, Cursor, Cline, VSCode, Warp). The codebase follows a clean layered architecture.

### Core Design Principles

1. **Single Source of Truth**: The canonical configuration at `~/Library/Application Support/mseep/canonical.json` is the single source of truth for all server definitions.

2. **Safe Merging**: Client adapters preserve unmanaged entries when updating configurations. Only mseep-managed servers are modified during sync operations.

3. **Automatic Backups**: All configuration changes create timestamped backups (`.bak.YYYYMMDD-HHMMSS` format) before modification.

4. **Fuzzy Matching**: The fuzzy matcher searches across names, aliases, and tags with intelligent scoring:
   - Exact match: 100 points
   - Contains match: 80 points (prefers shorter strings)
   - Token-based match: 60 points
   - Interactive prompts when multiple good matches exist (within 5 points)

### Key Components

**CLI Layer** (`cmd/mseep/`):
- `main.go`: Cobra command definitions and entry point
- `wire_cmd.go`: Command implementations that wire together app layer and adapters

**Application Logic** (`internal/app/`):
- `wire.go`: Core App struct that loads canonical config and orchestrates operations
- `apply.go`: Applies canonical state to detected clients with diff preview
- `status.go`: Shows enabled/disabled state across clients
- `health.go`: Runs health checks on enabled servers
- `profiles.go`: Manages server profiles for quick configuration switching

**Configuration System** (`internal/config/`):
- Manages canonical JSON schema with servers, profiles, and metadata
- Server fields: name, aliases, tags, command, args, env, transport, enabled, healthCheck, policy
- Profiles map profile names to lists of enabled server names
- `Load()` creates default empty config if file doesn't exist
- `Save()` updates timestamp and writes atomically

**Adapter Pattern** (`internal/adapters/`):
All adapters implement: `Name()`, `Path()`, `Detect()`, `Load()`, `Apply()`, `Backup()`, `Restore()`
- **Claude Desktop**: Manages `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Claude Code**: Claude Code CLI tool configuration
- **Cursor**: Manages Cursor's MCP configuration
- **Cline**: VSCode extension adapter
- **VSCode**: Native VSCode MCP support
- **Warp**: Terminal with MCP integration
- **Crush**: Crush AI client adapter
- **OpenCode**: OpenCode client adapter
- **LM Studio**: Manages `~/.cache/lm-studio/mcp-config.json`
- **Goose**: Manages `~/.config/goose/mcp-config.json`

**Fuzzy Matching** (`internal/fuzzy/`):
- Multi-field search with weighted scoring
- `SelectBest()` handles ambiguous matches with interactive user prompts
- Shows context (aliases, tags, scores) when presenting options
- Supports `--yes` flag to auto-select best match without prompting

**TUI** (`internal/tui/`):
- Built with Bubble Tea framework
- Five view modes: Servers, Profiles, Health, Apply, Marketplace
- Keybindings: t/space (toggle), a (apply), h (health), p (profiles), r (refresh), q (quit)
- Rich styling with lipgloss (Dracula-inspired color scheme)
- Server list shows enabled/disabled state, tags, command, aliases

**Marketplace** (`internal/marketplace/`):
- Aggregates MCP servers from multiple sources (mcpservers.org, GitHub awesome lists)
- 30-minute cache with deduplication by name
- `SearchServers()` for fuzzy search with relevance scoring
- `InstallServer()` adds marketplace entries to canonical config (disabled by default)

**Health Checks** (`internal/health/`):
- Three check types: stdio, http, tcp
- Configurable timeouts and retries per server
- Results include status (healthy/unhealthy/timeout/error), message, duration

**Diff Visualization** (`internal/diff/`):
- Generates unified diffs with color (green for additions, red for deletions)
- Used in apply command to preview changes before writing

### Configuration Files

**Canonical Config**: `~/Library/Application Support/mseep/canonical.json`
```json
{
  "servers": [
    {
      "name": "burp",
      "aliases": ["burp suite", "burpsuite"],
      "tags": ["security"],
      "command": "burp-mcp",
      "args": [],
      "env": {"BURP_API": "..."},
      "transport": "stdio",
      "enabled": false,
      "healthCheck": {"type": "stdio", "timeoutMs": 3000, "retries": 2},
      "policy": {"autoDisable": false}
    }
  ],
  "profiles": {"dev": ["burp", "github"]},
  "meta": {"version": "1", "updatedAt": "2025-01-15T..."}
}
```

**Client Configs**: Each client has its own format, but adapters normalize to/from canonical schema.

### Testing Strategy

- Unit tests for config load/save/marshal
- Unit tests for fuzzy matching with various queries
- Future: Golden tests for adapter merge operations
- Test files use `t.TempDir()` for isolated filesystem operations

### Error Handling

- Operations fail gracefully with descriptive errors
- Missing configs are created automatically (empty state)
- Adapter errors are collected but don't stop other adapters from running
- Health check failures are reported but don't affect enabled state (unless `--fix` is used)

### Platform Support

- Currently macOS-focused (uses `~/Library/Application Support` paths)
- Built for ARM64 (Apple Silicon) by default
- Client detection uses platform-specific paths