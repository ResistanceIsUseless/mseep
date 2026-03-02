package goose

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/diff"
)

// Goose AI config shape
// Path: ~/.config/goose/mcp-config.json

type GooseConfig struct {
	MCPServers map[string]GooseServer `json:"mcpServers"`
}

type GooseServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Adapter struct{}

func (Adapter) Name() string { return "goose" }

func (Adapter) Path() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", "goose", "mcp-config.json"), nil
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

func (a Adapter) Load() (*GooseConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &GooseConfig{MCPServers: map[string]GooseServer{}}, nil
		}
		return nil, err
	}
	var c GooseConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.MCPServers == nil {
		c.MCPServers = map[string]GooseServer{}
	}
	return &c, nil
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

// Apply merges canonical servers into Goose config, preserving unmanaged entries.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	cc, err := a.Load()
	if err != nil {
		return "", err
	}
	before, _ := json.MarshalIndent(cc, "", "  ")

	// build desired set from canonical enabled servers
	enabled := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled {
			enabled[s.Name] = s
		}
	}

	// start with existing; remove entries not in enabled
	newCfg := GooseConfig{MCPServers: map[string]GooseServer{}}
	for name, srv := range cc.MCPServers {
		if _, keep := enabled[name]; keep {
			newCfg.MCPServers[name] = srv
		}
	}
	// add/update enabled ones from canonical
	for name, s := range enabled {
		newCfg.MCPServers[name] = GooseServer{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
	}

	after, _ := json.MarshalIndent(newCfg, "", "  ")
	diffStr := diff.GenerateColorDiff(string(before), string(after))

	// Ensure directory exists
	p, err := a.Path()
	if err != nil {
		return diffStr, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return diffStr, err
	}

	// Write
	if _, err := a.Backup(); err != nil {
		return diffStr, err
	}
	if err := os.WriteFile(p, after, 0o644); err != nil {
		return diffStr, err
	}
	return diffStr, nil
}
