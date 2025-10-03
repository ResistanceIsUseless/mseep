# mseep

A fast Go TUI/CLI to manage MCP servers across clients (Claude Desktop first). Fuzzy quick commands, safe merges, backups, and opt‑in health checks. Marketplace via curated templates.

## Status
MVP CLI skeleton is in place: enable/disable/toggle with fuzzy best match, canonical config persistence, Claude adapter merge/write with backup.

## Install (local)
```bash
cd mseep
go build ./cmd/mseep
./mseep --help
```

## Usage (MVP)
```bash
# Enable by fuzzy name and apply to Claude if detected
./mseep enable burp

# Disable
./mseep disable "github"

# Toggle
./mseep toggle obsidian
```

## Canonical config
Stored at: `~/Library/Application Support/mseep/canonical.json` on macOS (uses UserConfigDir). When missing, a blank config is created.

Example schema snippet:
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
      "enabled": false,
      "healthCheck": {"type": "stdio", "timeoutMs": 3000, "retries": 2},
      "policy": {"autoDisable": false}
    }
  ],
  "profiles": {"dev": ["burp"]}
}
```

## Roadmap
- TUI (bubbletea) with diff preview, profiles, and status
- Status/health commands (manual, opt-in; no background daemon)
- Curated marketplace templates (no scraping, user reviewed)
- Additional adapters (Cursor/Cline)
- Golden tests for config merge

## License
MIT
