# mseep

A fast CLI/TUI for managing MCP (Model Context Protocol) servers across all your AI tools from a single canonical configuration. Toggle servers, switch profiles, run health checks, and browse a marketplace — all from one place.

## Features

- **Unified config** — manage MCP servers once, sync to all clients
- **iCloud sync** — sync your config across all your Macs
- **Toggle switches** — enable/disable servers with visual feedback
- **Quick profiles** — press 1-9 to instantly switch server profiles
- **Import existing** — discover and import servers from your current client configs
- **Safe merging** — never touches servers not managed by mseep

## Supported Clients

| Client | Config Path | Notes |
|--------|-------------|-------|
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | |
| Claude Code | `~/.claude.json` | |
| Cursor | `~/Library/Application Support/Cursor/User/settings.json` | JSONC support |
| VS Code | `~/Library/Application Support/Code/User/settings.json` | JSONC support |
| Cline | VSCode extension global storage | |
| Warp | SQLite database + `~/.warp/mcp_config.json` | Reads from Warp's internal DB |
| Crush | `~/.local/share/crush/crush.json` | |
| OpenCode | `~/.config/opencode/opencode.json` | |
| LM Studio | `~/.cache/lm-studio/mcp-config.json` | |
| Goose | `~/.config/goose/mcp-config.json` | |

All paths are platform-aware (macOS, Linux, Windows).

## Install

### Using go install (recommended)

```bash
go install github.com/ResistanceIsUseless/mseep/cmd/mseep@latest
```

### From source

```bash
git clone https://github.com/ResistanceIsUseless/mseep
cd mseep
go build ./cmd/mseep
./mseep --help
```

## Quick Start

```bash
# Import existing MCP servers from your clients
mseep import --dry-run    # preview what would be imported
mseep import              # import all discovered servers

# Enable iCloud sync (macOS)
mseep sync enable

# Launch the TUI
mseep tui
```

## TUI

```bash
mseep tui
```

```
┌─────────────────────────────────────────────────────────────────┐
│  mseep                                          Mode: BASIC     │
├─────────────────────────────────────────────────────────────────┤
│  Servers │ Profiles │ Status │ Health │ Apply │ Marketplace    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  [ON ]  burp                                         stdio      │
│         Burp Suite MCP proxy for web security testing           │
│         security, web                                           │
│                                                                 │
│  [OFF]  kubernetes                                   stdio      │
│         Kubernetes cluster management via MCP                   │
│         devops, k8s                                             │
│                                                                 │
│  [OFF]  playwright                                   stdio      │
│         Browser automation with Playwright                      │
│         automation, browser                                     │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ 📦 1/10 enabled · space: toggle · a: apply · 1:automation ...   │
└─────────────────────────────────────────────────────────────────┘
```

### Key Bindings

| Key | Action |
|-----|--------|
| `Space` / `Enter` / `t` | Toggle selected server |
| `1-9` | Quick-apply profile (shown in status bar) |
| `a` | Apply changes to all clients |
| `m` | Toggle delivery mode (basic/wrapper) |
| `r` | Refresh |
| `←` / `→` / `Tab` | Switch tabs |
| `?` | Help |
| `q` | Quit |

### Tabs

| Tab | Description |
|-----|-------------|
| **Servers** | Toggle servers on/off, see descriptions and tags |
| **Profiles** | Apply saved server profiles |
| **Status** | Client sync status (which servers are synced to which clients) |
| **Health** | Run health checks against enabled servers |
| **Apply** | Apply canonical config to all detected clients |
| **Marketplace** | Browse and install servers from mcpservers.org |

## CLI Reference

```bash
# Server management
mseep servers list                    # list all servers with status
mseep servers add myserver            # add a new server interactively
mseep servers show burp               # show server details
mseep servers remove oldserver        # remove a server

# Toggle servers
mseep enable "github"                 # fuzzy match and enable
mseep disable "burp" --yes            # disable without prompt
mseep toggle kubernetes               # toggle state

# Import from existing clients
mseep import --dry-run                # preview imports
mseep import --client warp            # import from specific client
mseep import --enable                 # enable imported servers immediately

# Profiles
mseep profiles list
mseep profiles create security burp semgrep ghidra
mseep profiles save mysession         # save current enabled servers
mseep profiles apply security         # apply a profile

# Status and health
mseep status                          # show all clients and sync state
mseep status --client claude --json   # JSON output for scripting
mseep health                          # run health checks
mseep health --fix                    # auto-disable failing servers

# Apply to clients
mseep apply                           # apply to all detected clients
mseep apply --client cursor           # apply to specific client
mseep apply -y                        # skip confirmation

# iCloud sync (macOS)
mseep sync status                     # check sync status
mseep sync enable                     # enable iCloud sync
mseep sync disable                    # disable sync, keep local copy

# Delivery mode
mseep mode get                        # show current mode
mseep mode set wrapper                # use mseep as proxy
mseep mode set basic                  # write servers directly to clients
```

## Delivery Modes

### Basic (default)
Each enabled server is written directly into every detected client's config file. Simple, no runtime dependency on mseep.

### Wrapper
Each client is configured with a single `mseep proxy` entry. mseep multiplexes all enabled servers at runtime — enabling hot-reload and unified tool namespacing without restarting clients.

```
Claude Code ──stdio──▶ mseep proxy ──stdio──▶ ghidra MCP
                                   ──stdio──▶ burp MCP
                                   ──http──▶  semgrep MCP

Cursor      ──stdio──▶ mseep proxy ──stdio──▶ (isolated processes)
```

## Canonical Config

Single source of truth at `~/Library/Application Support/mseep/canonical.json` (synced to iCloud if enabled).

```json
{
  "servers": [
    {
      "name": "burp",
      "command": "/path/to/java",
      "args": ["-jar", "mcp-proxy.jar"],
      "transport": "stdio",
      "enabled": true,
      "description": "Burp Suite MCP proxy for web security testing",
      "tags": ["security", "web"],
      "secrets": [
        {"name": "BURP_API_KEY", "description": "API key for Burp", "required": false}
      ]
    }
  ],
  "profiles": {
    "security": ["burp", "semgrep", "ghidra"],
    "automation": ["playwright", "browser"],
    "devops": ["kubernetes", "portainer"]
  },
  "settings": {
    "mode": "basic"
  }
}
```

## iCloud Sync

Enable iCloud sync to keep your MCP server configuration synchronized across all your Macs:

```bash
mseep sync enable
```

This moves your canonical config to iCloud Drive and creates a symlink:
- **Config location**: `~/Library/Mobile Documents/com~apple~CloudDocs/mseep/canonical.json`
- **Symlink**: `~/Library/Application Support/mseep/canonical.json`

## Architecture

```
cmd/mseep/           CLI entry point (Cobra commands)
internal/
  adapters/          Client adapters (claude, cursor, vscode, warp, etc.)
  app/               Core business logic (Apply, Status, Health, Toggle)
  config/            Canonical config schema and persistence
  diff/              Unified diff generation
  fuzzy/             Fuzzy matching with scoring
  health/            Health check implementations
  marketplace/       Server discovery from external sources
  proxy/             MCP multiplexer for wrapper mode
  style/             Lipgloss terminal styling
  tui/               Bubble Tea TUI application
  jsonc/             JSON with comments parser (for VS Code/Cursor)
```

## License

MIT
