# mseep

A fast CLI/TUI for managing MCP (Model Context Protocol) servers across all your AI tools from a single canonical configuration. Toggle servers, switch profiles, run health checks, and browse a marketplace — all from one place.

## Supported Clients

| Client | Config Path |
|--------|-------------|
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Claude Code | `~/.claude.json` |
| Cursor | `~/Library/Application Support/Cursor/User/settings.json` |
| VS Code | `~/Library/Application Support/Code/User/settings.json` |
| Cline | VSCode extension global storage |
| Warp | `~/.warp/mcp_config.json` |
| Crush | `~/.local/share/crush/crush.json` |
| OpenCode | `~/.config/opencode/opencode.json` |

All paths are platform-aware (macOS, Linux, Windows).

## Delivery Modes

### Basic (default)
Each enabled server is written directly into every detected client's config file. Simple, no runtime dependency on mseep.

### Wrapper
Each client is configured with a single `mseep proxy` entry. mseep multiplexes all enabled servers at runtime — enabling hot-reload and unified tool namespacing without restarting clients.

```
Claude Code ──stdio──▶ mseep proxy ──stdio──▶ ghidra MCP process
                                   ──stdio──▶ burp MCP process
                                   ──http──▶  semgrep MCP endpoint

Cursor      ──stdio──▶ mseep proxy ──stdio──▶ ghidra MCP process (isolated)
                                   ──stdio──▶ burp MCP process   (isolated)
```

Each client's proxy process is fully isolated — separate OS process trees, no shared session state.

Switch modes:
```bash
mseep mode set wrapper   # switch to wrapper mode
mseep mode set basic     # switch back to basic
mseep mode get           # show current mode and settings
```

## Install

```bash
git clone https://github.com/ResistanceIsUseless/mseep
cd mseep
go build ./cmd/mseep
./mseep --help
```

## TUI (primary interface)

```bash
./mseep tui
```

| Tab | Key | Description |
|-----|-----|-------------|
| Servers | `t`/`space` | Toggle server enabled/disabled |
| Profiles | `↵` | Apply a saved profile |
| **Status** | — | Client sync status table (all 8 clients) |
| Health | `H` | Run health checks against enabled servers |
| Apply | `a` | Apply canonical to all detected clients |
| Marketplace | `↵` | Install a server from the marketplace |

**Global keys:** `m` toggle mode · `a` apply · `r` refresh · `?` help · `q` quit · `←`/`→` switch tabs

The mode badge (top-right) shows `BASIC` or `WRAPPER` at all times.

## CLI Quick Reference

```bash
# Toggle servers
./mseep enable "github"       # fuzzy match, enable, apply to detected clients
./mseep disable "burp" --yes  # auto-select if ambiguous
./mseep toggle obsidian

# Status and health
./mseep status                # show all clients and server sync state
./mseep status --client claude-code --json
./mseep health                # run health checks (manual, opt-in)
./mseep health --fix          # auto-disable failing servers

# Apply
./mseep apply                          # apply to all detected clients
./mseep apply --client cursor          # apply to one client
./mseep apply --profile security       # apply a profile, then sync

# Profiles
./mseep profiles list
./mseep profiles create security burp semgrep ghidra
./mseep profiles save mysession
./mseep profiles apply security

# Mode
./mseep mode get
./mseep mode set wrapper --poll 10 --namespace=true
./mseep mode set basic
```

## Canonical Config

Single source of truth at `~/.config/mseep/canonical.json` (macOS: `~/Library/Application Support/mseep/canonical.json`).

```json
{
  "servers": [
    {
      "name": "burp",
      "aliases": ["burp suite", "burpsuite"],
      "tags": ["security", "web"],
      "command": "burp-mcp",
      "args": [],
      "env": {"BURP_API_KEY": "..."},
      "transport": "stdio",
      "enabled": false,
      "healthCheck": {"type": "stdio", "timeoutMs": 3000, "retries": 2},
      "policy": {"autoDisable": false}
    }
  ],
  "profiles": {
    "security": ["burp", "semgrep", "ghidra"]
  },
  "settings": {
    "mode": "basic",
    "wrapper": {
      "pollIntervalSeconds": 5,
      "namespaceTools": true,
      "namespaceSeparator": "__"
    }
  }
}
```

## Wrapper Mode Details

When mode is `wrapper`, running `mseep apply` writes a single entry into each client config:

```json
{
  "mcpServers": {
    "mseep": {
      "command": "/usr/local/bin/mseep",
      "args": ["proxy", "--client", "claude-code"]
    }
  }
}
```

The proxy (`mseep proxy --client <name>`) then:
- Spawns a subprocess for each enabled stdio MCP server
- Aggregates their tools, resources, and prompts into one namespace
- Prefixes tool names to avoid collisions (`ghidra__analyze`, `semgrep__scan`)
- Polls `canonical.json` every N seconds — newly enabled/disabled servers are started/stopped without restarting the client
- Each client gets its own isolated proxy process and child tree

HTTP/SSE transport servers are forwarded directly without subprocess management.

## Safe Merge Behaviour

Servers not managed by mseep (not in canonical) are **never touched** when applying. Only servers known to canonical are added, updated, or removed. All config writes create timestamped backups (`.bak.YYYYMMDD-HHMMSS`).

## Architecture

```
cmd/mseep/
  main.go          CLI root (Cobra)
  wire_cmd.go      Command implementations
  mode_cmd.go      mseep mode get|set
  proxy_cmd.go     mseep proxy --client <name>

internal/
  adapters/        One adapter per client (claude, claude-code, cursor,
                   vscode, cline, warp, crush, opencode)
  app/             Business logic (apply, status, health, profiles)
  config/          Canonical JSON schema + load/save
  proxy/           MCP JSON-RPC multiplexer (wrapper mode)
  fuzzy/           Multi-field fuzzy search with interactive disambiguation
  health/          stdio/http/tcp health checkers
  marketplace/     Server discovery from mcpservers.org and GitHub
  diff/            Coloured unified diff for config preview
  tui/             Bubble Tea TUI (6 tabs)
  style/           Lipgloss styling (Dracula palette)
```

## License

MIT
