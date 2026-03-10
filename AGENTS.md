# AGENTS.md

Guidelines for AI coding agents working in the mseep codebase.

## Build & Test Commands

```bash
# Build
go build ./cmd/mseep              # Build binary to ./mseep
./mseep --help                    # Verify build

# Test all packages
go test ./...

# Test specific package
go test ./internal/config
go test ./internal/fuzzy

# Run a single test by name
go test ./internal/config -run TestLoadSave
go test ./internal/fuzzy -run TestCandidates
go test ./internal/fuzzy -run "TestSelectBest/exact_match_returns_immediately"

# Verbose output
go test -v ./internal/config -run TestLoadSave

# Dependency management
go mod tidy
```

## Project Structure

```
cmd/mseep/           # CLI entry point (Cobra commands)
internal/
  adapters/          # Client adapters (claude, cursor, vscode, etc.)
  app/               # Core business logic (Apply, Status, Health, Toggle)
  config/            # Canonical config schema and persistence
  diff/              # Unified diff generation
  fuzzy/             # Fuzzy matching with scoring
  health/            # Health check implementations
  marketplace/       # Server discovery from external sources
  proxy/             # MCP multiplexer for wrapper mode
  style/             # Lipgloss terminal styling
  tui/               # Bubble Tea TUI application
```

## Code Style

### Imports

Group imports in this order with blank lines between groups:

```go
import (
    // Standard library
    "fmt"
    "os"

    // External dependencies
    "github.com/spf13/cobra"

    // Internal packages
    "github.com/ResistanceIsUseless/mseep/internal/config"
)
```

### Naming Conventions

- **Packages**: lowercase, single word (`config`, `fuzzy`, `health`)
- **Exported types**: PascalCase (`Canonical`, `ServerEntry`, `CheckResult`)
- **Unexported types/functions**: camelCase (`scoreOne`, `buildProfileItems`)
- **Interfaces**: verb-based noun (`Checker`, `Source`, `Adapter`)
- **Adapters**: struct named `Adapter` with receiver methods

### Error Handling

Compact single-line returns are acceptable:

```go
if err != nil { return nil, err }
if err != nil { return "", err }
```

Multi-line for complex logic. Always wrap errors with context using `%w`:

```go
if err != nil {
    return nil, fmt.Errorf("loading config: %w", err)
}
```

### Struct Tags

Use `json` tags with `omitempty` for optional fields. Example: `Aliases []string \`json:"aliases,omitempty"\``

### Testing Patterns

- Use table-driven tests with `t.Run(tt.name, func(t *testing.T) {...})`
- Use `t.TempDir()` for file system isolation
- `t.Errorf()` for non-fatal assertions, `t.Fatalf()` for fatal ones

## Architecture Notes

### Adapter Interface

All client adapters implement:

```go
type Adapter interface {
    Name() string
    Path() (string, error)
    Detect() (bool, error)
    Load() (*ClientConfig, error)
    Apply(*config.Canonical) (string, error)
    Backup() (string, error)
    Restore(path string) error
}
```

### Key Paths

- Canonical config: `~/Library/Application Support/mseep/canonical.json`
- Client configs vary by adapter (see individual adapter files)
- Backups: `.bak.YYYYMMDD-HHMMSS` format alongside original

### Design Principles

1. **Single Source of Truth**: Canonical config is authoritative
2. **Safe Merging**: Preserve unmanaged entries in client configs
3. **Automatic Backups**: Create timestamped backups before any write
4. **Graceful Degradation**: Log errors but continue processing other items

## Common Patterns

### Loading the App

```go
app, err := app.LoadApp()
if err != nil { return err }
// app.Canon contains the canonical configuration
```

### Fuzzy Matching

```go
idx := make([]fuzzy.Index, 0, len(servers))
for _, s := range servers {
    idx = append(idx, fuzzy.Index{Name: s.Name, Aliases: s.Aliases, Tags: s.Tags})
}
match, err := fuzzy.SelectBest(query, idx, assumeYes)
```

## Dependencies

- CLI: `github.com/spf13/cobra`
- TUI: `github.com/charmbracelet/bubbletea`, `bubbles`, `lipgloss`
- Diff: `github.com/sergi/go-diff`
- Go version: 1.24.2
