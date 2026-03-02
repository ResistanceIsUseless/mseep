package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/diff"
)

// Claude Code stores MCP configuration in ~/.claude.json (user scope).
//
// Schema mirrors Claude Desktop's mcpServers map, with an additional optional
// "type" field ("stdio" | "http" | "sse") that Claude Code uses to distinguish
// transport. We write "type":"stdio" for stdio servers and omit for http/sse
// since those use a "url" field instead of command/args.
//
// We manage the user-scope file only. Project-scope (.mcp.json in repo roots)
// is intentionally out of scope — those are team-committed files.

// ClaudeCodeConfig is the top-level ~/.claude.json shape (subset we care about).
// Other keys (projects, auth, preferences, etc.) are preserved via the rawOther
// field during load/save.
type ClaudeCodeConfig struct {
	MCPServers map[string]ClaudeCodeServer `json:"mcpServers"`
}

// ClaudeCodeServer represents a single MCP server entry in Claude Code's config.
type ClaudeCodeServer struct {
	// Type hints the transport; Claude Code uses "stdio" | "http" | "sse".
	// Omitted if empty (Claude Code defaults to stdio when command is present).
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// URL is used for http/sse transport instead of command/args.
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Adapter implements the mseep adapter interface for Claude Code.
type Adapter struct{}

func (Adapter) Name() string { return "claude-code" }

// Path returns the user-scope Claude Code config path.
// macOS/Linux: ~/.claude.json
// Windows:     %USERPROFILE%\.claude.json
func (Adapter) Path() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".claude.json"), nil
}

func (a Adapter) Detect() (bool, error) {
	p, err := a.Path()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Load reads the Claude Code config. If the file does not exist an empty
// config is returned (Claude Code will create it on first run).
// We unmarshal only mcpServers; all other top-level keys are left untouched
// by preserving the raw JSON for the write path.
func (a Adapter) Load() (*ClaudeCodeConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &ClaudeCodeConfig{MCPServers: map[string]ClaudeCodeServer{}}, nil
		}
		return nil, err
	}
	var c ClaudeCodeConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.MCPServers == nil {
		c.MCPServers = map[string]ClaudeCodeServer{}
	}
	return &c, nil
}

// loadRaw reads the full file as a generic map so we can preserve fields we
// don't model (projects, auth, etc.) when writing back.
func (a Adapter) loadRaw() (map[string]json.RawMessage, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		// File might be empty or malformed; start fresh.
		return map[string]json.RawMessage{}, nil
	}
	return raw, nil
}

func (a Adapter) Backup() (string, error) {
	p, err := a.Path()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	bak := p + ".bak." + time.Now().Format("20060102-150405")
	if err := os.WriteFile(bak, b, 0o600); err != nil {
		return "", err
	}
	return bak, nil
}

func (a Adapter) Restore(path string) error {
	p, err := a.Path()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// Apply merges canonical enabled servers into Claude Code's config.
//
// Safe-merge behaviour:
//   - Servers present in canonical (enabled or disabled) that exist in the
//     client config are either updated (if enabled) or removed (if disabled).
//   - Servers NOT in canonical at all are left untouched (unmanaged entries).
//   - All non-mcpServers keys in ~/.claude.json are preserved verbatim.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	// Load existing config for diff baseline.
	cc, err := a.Load()
	if err != nil {
		return "", err
	}
	before, _ := json.MarshalIndent(cc, "", "  ")

	// Load raw file to preserve non-mcpServers fields on write.
	raw, err := a.loadRaw()
	if err != nil {
		return "", err
	}

	// Build the desired mcpServers map.
	enabled := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled {
			enabled[s.Name] = s
		}
	}

	// Index canonical names for safe-merge (to know which entries are "managed").
	managed := map[string]bool{}
	for _, s := range canon.Servers {
		managed[s.Name] = true
	}

	newServers := map[string]ClaudeCodeServer{}

	// Keep unmanaged entries as-is.
	for name, srv := range cc.MCPServers {
		if !managed[name] {
			newServers[name] = srv
		}
	}

	// Add enabled entries from canonical.
	for name, s := range enabled {
		newServers[name] = canonicalToClaudeCode(s)
	}

	// Build the after state for diff display.
	after := &ClaudeCodeConfig{MCPServers: newServers}
	afterJSON, _ := json.MarshalIndent(after, "", "  ")
	diffStr := diff.GenerateColorDiff(string(before), string(afterJSON))

	// Merge updated mcpServers back into the raw map and write.
	mcpRaw, err := json.Marshal(newServers)
	if err != nil {
		return diffStr, err
	}
	raw["mcpServers"] = json.RawMessage(mcpRaw)

	outBytes, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return diffStr, err
	}

	if _, err := a.Backup(); err != nil {
		return diffStr, err
	}
	p, err := a.Path()
	if err != nil {
		return diffStr, err
	}
	// ~/.claude.json may contain auth tokens — keep it user-readable only.
	if err := os.WriteFile(p, outBytes, 0o600); err != nil {
		return diffStr, err
	}
	return diffStr, nil
}

// canonicalToClaudeCode converts a canonical server definition to Claude Code's
// config format. HTTP/SSE servers use the url field; stdio servers use command/args.
func canonicalToClaudeCode(s config.Server) ClaudeCodeServer {
	switch s.Transport {
	case "http", "sse":
		return ClaudeCodeServer{
			Type: s.Transport,
			URL:  s.Command, // For http/sse, canonical stores the URL in Command.
			Env:  s.Env,
		}
	default: // stdio
		return ClaudeCodeServer{
			Type:    "stdio",
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
	}
}
