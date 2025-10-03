package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mseep/internal/config"
)

// Minimal Claude Desktop config shape (subset)
// Path: ~/Library/Application Support/Claude/claude_desktop_config.json

type ClaudeConfig struct {
	MCPServers map[string]ClaudeServer `json:"mcpServers"`
}

type ClaudeServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Adapter struct{}

func (Adapter) Name() string { return "claude" }

func (Adapter) Path() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil { return "", err }
	return filepath.Join(h, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
}

func (a Adapter) Detect() (bool, error) {
	p, err := a.Path(); if err != nil { return false, err }
	_, err = os.Stat(p)
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return false, err
}

func (a Adapter) Load() (*ClaudeConfig, error) {
	p, err := a.Path(); if err != nil { return nil, err }
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) { return &ClaudeConfig{MCPServers: map[string]ClaudeServer{}}, nil }
		return nil, err
	}
	var c ClaudeConfig
	if err := json.Unmarshal(b, &c); err != nil { return nil, err }
	if c.MCPServers == nil { c.MCPServers = map[string]ClaudeServer{} }
	return &c, nil
}

func (a Adapter) Backup() (string, error) {
	p, err := a.Path(); if err != nil { return "", err }
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) { return "", nil }
		return "", err
	}
	bak := p + ".bak." + time.Now().Format("20060102-150405")
	if err := os.WriteFile(bak, b, 0o644); err != nil { return "", err }
	return bak, nil
}

func (a Adapter) Restore(path string) error {
	p, err := a.Path(); if err != nil { return err }
	b, err := os.ReadFile(path); if err != nil { return err }
	return os.WriteFile(p, b, 0o644)
}

// Apply merges canonical servers into Claude config, preserving unmanaged entries.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	cc, err := a.Load(); if err != nil { return "", err }
	before, _ := json.MarshalIndent(cc, "", "  ")

	// build desired set from canonical enabled servers
	enabled := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled {
			enabled[s.Name] = s
		}
	}

	// start with existing; remove entries not in enabled
	newCfg := ClaudeConfig{MCPServers: map[string]ClaudeServer{}}
	for name, srv := range cc.MCPServers {
		if _, keep := enabled[name]; keep {
			newCfg.MCPServers[name] = srv
		}
	}
	// add/update enabled ones from canonical
	for name, s := range enabled {
		newCfg.MCPServers[name] = ClaudeServer{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
	}

	after, _ := json.MarshalIndent(newCfg, "", "  ")
	diff := unifiedDiff(string(before), string(after))

	// Write
	if _, err := a.Backup(); err != nil { return diff, err }
	p, err := a.Path(); if err != nil { return diff, err }
	if err := os.WriteFile(p, after, 0o644); err != nil { return diff, err }
	return diff, nil
}

// naive diff for MVP
func unifiedDiff(a, b string) string {
	return fmt.Sprintf("--- before\n+++ after\n%s\n%s", a, b)
}
