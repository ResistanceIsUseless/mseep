package crush

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/diff"
)

// Crush (charmbracelet/crush) stores MCP configuration under its state directory.
//
// Config path:
//   - macOS/Linux: $XDG_DATA_HOME/crush/crush.json  (falls back to ~/.local/share/crush/crush.json)
//   - Windows:     %LOCALAPPDATA%\crush\crush.json
//
// Crush's MCP schema uses a top-level "mcp" key (not "mcpServers").
//
// Notable schema differences from Claude Desktop:
//   - Uses "disabled": true/false (inverted bool) rather than "enabled"
//   - Supports per-server "timeout" (seconds)
//   - Supports "disabled_tools" []string for selectively hiding tools
//   - Transport specified via "type": "stdio"|"http"|"sse"
//   - For http/sse, a "url" field is used instead of command/args
//
// All non-"mcp" keys in crush.json are preserved verbatim on write.

// CrushConfig is the top-level crush.json shape (mcp section only).
type CrushConfig struct {
	MCP map[string]CrushServer `json:"mcp"`
}

// CrushServer is a single MCP server entry in Crush's config format.
type CrushServer struct {
	// Type is "stdio" | "http" | "sse". Crush defaults to stdio if omitted.
	Type string `json:"type,omitempty"`

	// Command and Args for stdio transport.
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// URL for http/sse transport.
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	Env map[string]string `json:"env,omitempty"`

	// Disabled inverts the enabled semantics: true = server is off.
	// We always write false for servers we manage (they are enabled by definition).
	Disabled bool `json:"disabled,omitempty"`

	// Timeout is an optional per-server timeout in seconds.
	Timeout int `json:"timeout,omitempty"`

	// DisabledTools optionally hides specific tools from the client.
	// mseep does not manage this field; it is preserved from existing config.
	DisabledTools []string `json:"disabled_tools,omitempty"`
}

// Adapter implements the mseep adapter interface for Crush.
type Adapter struct{}

func (Adapter) Name() string { return "crush" }

// Path returns the platform-appropriate crush.json path.
func (Adapter) Path() (string, error) {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA environment variable not set")
		}
		return filepath.Join(localAppData, "crush", "crush.json"), nil
	default: // darwin, linux
		// Honour XDG_DATA_HOME if set; fall back to ~/.local/share
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dataHome = filepath.Join(h, ".local", "share")
		}
		return filepath.Join(dataHome, "crush", "crush.json"), nil
	}
}

func (a Adapter) Detect() (bool, error) {
	p, err := a.Path()
	if err != nil {
		return false, err
	}
	// Detect on config file existence; the directory alone is insufficient since
	// the XDG data dir always exists on Linux.
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Load reads the Crush MCP config section.
func (a Adapter) Load() (*CrushConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &CrushConfig{MCP: map[string]CrushServer{}}, nil
		}
		return nil, err
	}
	var c CrushConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.MCP == nil {
		c.MCP = map[string]CrushServer{}
	}
	return &c, nil
}

// loadRaw reads crush.json as a generic map to preserve non-mcp keys.
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
	if err := os.WriteFile(bak, b, 0o644); err != nil {
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
	return os.WriteFile(p, b, 0o644)
}

// Apply merges canonical enabled servers into Crush's config using safe-merge.
//
// Key behaviour notes:
//   - Crush uses "disabled":true to suppress a server; we always write disabled:false
//     for servers we manage (they are enabled by definition in the enabled set).
//   - Per-server DisabledTools from an existing entry is preserved on update.
//   - Unmanaged entries (not in canonical) are left untouched.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	cc, err := a.Load()
	if err != nil {
		return "", err
	}
	before, _ := json.MarshalIndent(cc, "", "  ")

	raw, err := a.loadRaw()
	if err != nil {
		return "", err
	}

	// Build desired enabled set.
	enabled := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled {
			enabled[s.Name] = s
		}
	}

	// Index all canonical names (managed = in canonical regardless of enabled state).
	managed := map[string]bool{}
	for _, s := range canon.Servers {
		managed[s.Name] = true
	}

	newMCP := map[string]CrushServer{}

	// Preserve unmanaged entries verbatim.
	for name, srv := range cc.MCP {
		if !managed[name] {
			newMCP[name] = srv
		}
	}

	// Add/update enabled entries; preserve DisabledTools from existing entry.
	for name, s := range enabled {
		entry := canonicalToCrush(s)
		if existing, ok := cc.MCP[name]; ok {
			entry.DisabledTools = existing.DisabledTools
		}
		newMCP[name] = entry
	}

	// Build after state for diff.
	after := &CrushConfig{MCP: newMCP}
	afterJSON, _ := json.MarshalIndent(after, "", "  ")
	diffStr := diff.GenerateColorDiff(string(before), string(afterJSON))

	// Merge back into raw map.
	mcpRaw, err := json.Marshal(newMCP)
	if err != nil {
		return diffStr, err
	}
	raw["mcp"] = json.RawMessage(mcpRaw)

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
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return diffStr, err
	}
	if err := os.WriteFile(p, outBytes, 0o644); err != nil {
		return diffStr, err
	}
	return diffStr, nil
}

// canonicalToCrush converts a canonical server to Crush's config format.
// Note: disabled is always false since we only write enabled servers.
func canonicalToCrush(s config.Server) CrushServer {
	switch s.Transport {
	case "http", "sse":
		return CrushServer{
			Type:     s.Transport,
			URL:      s.Command, // HTTP/SSE stores URL in Command field canonically.
			Env:      s.Env,
			Disabled: false,
		}
	default: // stdio
		return CrushServer{
			Type:     "stdio",
			Command:  s.Command,
			Args:     s.Args,
			Env:      s.Env,
			Disabled: false,
		}
	}
}
