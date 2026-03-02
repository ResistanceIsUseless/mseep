package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"mseep/internal/config"
	"mseep/internal/diff"
)

// OpenCode stores global MCP configuration in:
//   - macOS/Linux: ~/.config/opencode/opencode.json
//   - Windows:     %APPDATA%\opencode\opencode.json
//
// A project-level opencode.json at the repo root takes precedence at runtime,
// but mseep manages the global config only — project configs are repo-owned.
//
// OpenCode's MCP schema uses a top-level "mcp" key with these quirks vs canonical:
//   - "type": "local" (stdio) | "remote" (http/sse)  — not "stdio"/"http"
//   - "command" is a []string (first element = binary, rest = args),
//      NOT separate "command" + "args" fields
//   - "environment" for env vars (not "env")
//   - No top-level "enabled" field; presence = enabled

// OpenCodeConfig is the top-level opencode.json shape (mcp section only).
type OpenCodeConfig struct {
	MCP map[string]OpenCodeServer `json:"mcp"`
}

// OpenCodeServer is a single MCP entry in OpenCode's format.
type OpenCodeServer struct {
	// Type is "local" (stdio) or "remote" (http/sse).
	Type string `json:"type"`

	// Command is [binary, arg1, arg2, ...] for local (stdio) servers.
	Command []string `json:"command,omitempty"`

	// URL for remote (http/sse) servers.
	URL string `json:"url,omitempty"`

	// Headers for remote servers (e.g. Authorization).
	Headers map[string]string `json:"headers,omitempty"`

	// Environment maps env var names to values.
	Environment map[string]string `json:"environment,omitempty"`
}

// Adapter implements the mseep adapter interface for OpenCode.
type Adapter struct{}

func (Adapter) Name() string { return "opencode" }

// Path returns the global opencode.json config path.
func (Adapter) Path() (string, error) {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "opencode", "opencode.json"), nil
	default: // darwin, linux
		// Honour XDG_CONFIG_HOME if set; fall back to ~/.config
		cfgHome := os.Getenv("XDG_CONFIG_HOME")
		if cfgHome == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			cfgHome = filepath.Join(h, ".config")
		}
		return filepath.Join(cfgHome, "opencode", "opencode.json"), nil
	}
}

func (a Adapter) Detect() (bool, error) {
	p, err := a.Path()
	if err != nil {
		return false, err
	}
	// Detect on config directory existence — opencode creates the dir on first run
	// even before the config file is written.
	if _, err := os.Stat(filepath.Dir(p)); err == nil {
		return true, nil
	}
	// Fall back to checking the config file itself.
	if _, err := os.Stat(p); err == nil {
		return true, nil
	}
	return false, nil
}

// Load reads the OpenCode MCP config section.
func (a Adapter) Load() (*OpenCodeConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &OpenCodeConfig{MCP: map[string]OpenCodeServer{}}, nil
		}
		return nil, err
	}
	var c OpenCodeConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.MCP == nil {
		c.MCP = map[string]OpenCodeServer{}
	}
	return &c, nil
}

// loadRaw reads opencode.json as a generic map to preserve non-mcp keys
// (provider credentials, theme, keybindings, etc.).
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

// Apply merges canonical enabled servers into OpenCode's global config using safe-merge.
//
// Schema translation:
//   - canonical transport "stdio" (or empty) → OpenCode type "local", command []string
//   - canonical transport "http"|"sse"        → OpenCode type "remote", url field
//   - canonical env map → OpenCode "environment" map
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

	// Index all canonical names to distinguish managed vs unmanaged entries.
	managed := map[string]bool{}
	for _, s := range canon.Servers {
		managed[s.Name] = true
	}

	newMCP := map[string]OpenCodeServer{}

	// Preserve unmanaged entries verbatim.
	for name, srv := range cc.MCP {
		if !managed[name] {
			newMCP[name] = srv
		}
	}

	// Add/update enabled entries.
	for name, s := range enabled {
		newMCP[name] = canonicalToOpenCode(s)
	}

	// Build after state for diff.
	after := &OpenCodeConfig{MCP: newMCP}
	afterJSON, _ := json.MarshalIndent(after, "", "  ")
	diffStr := diff.GenerateColorDiff(string(before), string(afterJSON))

	// Merge updated mcp section back into the raw map.
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

// canonicalToOpenCode translates a canonical server to OpenCode's format.
//
// Canonical command/args → OpenCode command []string (concatenated).
// Canonical transport "http"|"sse" → type "remote" with url.
// Canonical env → environment.
func canonicalToOpenCode(s config.Server) OpenCodeServer {
	switch s.Transport {
	case "http", "sse":
		return OpenCodeServer{
			Type:        "remote",
			URL:         s.Command, // HTTP/SSE: canonical stores URL in Command field.
			Environment: s.Env,
		}
	default: // stdio
		// OpenCode uses command []string = [binary, ...args]
		cmd := []string{s.Command}
		cmd = append(cmd, s.Args...)
		return OpenCodeServer{
			Type:        "local",
			Command:     cmd,
			Environment: s.Env,
		}
	}
}
