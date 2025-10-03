# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the binary
go build ./cmd/mseep

# Run the compiled binary
./mseep --help

# Clean up dependencies
go mod tidy
```

### CLI Usage Examples
```bash
./mseep enable "github"     # Enable a server by fuzzy search
./mseep disable "burp"      # Disable a server by fuzzy search  
./mseep toggle "obsidian"   # Toggle a server by fuzzy search
```

## Architecture

This is a Go-based CLI/TUI tool for managing MCP (Model Context Protocol) servers across different clients. The codebase follows a layered architecture with clear separation of concerns.

### Core Components

1. **CLI Layer** (`cmd/mseep/`): Cobra-based CLI with command handlers. The main entry point defines commands, while wire_cmd.go contains the actual implementations.

2. **Application Logic** (`internal/app/`): Core business logic that orchestrates operations between config management and client adapters.

3. **Configuration System** (`internal/config/`): 
   - Manages canonical configuration at `~/Library/Application Support/mseep/canonical.json`
   - Supports server definitions with names, aliases, tags, commands, and health checks
   - Implements automatic backup before any changes

4. **Adapter Pattern** (`internal/adapters/`):
   - Claude Desktop adapter manages `~/Library/Application Support/Claude/claude_desktop_config.json`
   - Uses safe merging that preserves unmanaged entries while updating mseep-managed servers
   - Creates timestamped backups before modifications

5. **Fuzzy Matching** (`internal/fuzzy/`): 
   - Multi-field search across server names, aliases, and tags
   - Scoring system: exact match (100), contains (80), token-based (60)
   - Auto-selects best match in MVP mode

### Key Implementation Details

- **Configuration Safety**: All config changes create automatic backups with `.bak.YYYYMMDD-HHMMSS` format
- **Merge Strategy**: The Claude adapter preserves existing non-mseep entries while updating managed servers
- **Error Handling**: Operations fail gracefully with descriptive error messages when configs are missing
- **Platform**: Built for ARM64 (Apple Silicon)

### Current Status

**Implemented (MVP)**:
- Enable/disable/toggle commands with fuzzy matching
- Claude Desktop integration with safe config merging
- Automatic backup system
- Canonical configuration management

**Not Yet Implemented**:
- TUI interface (bubbletea framework)
- Status, health, and apply commands
- Additional client adapters (Cursor, Cline)
- Test suite