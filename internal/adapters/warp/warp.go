package warp

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

// Warp MCP config shape
// Warp stores MCP configuration in its settings
// Path varies by platform:
// - macOS: ~/.warp/mcp_config.json
// - Linux: ~/.config/warp/mcp_config.json
// - Windows: %APPDATA%\warp\mcp_config.json

type WarpConfig struct {
	MCPServers map[string]WarpServer `json:"mcp_servers"`
}

type WarpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled,omitempty"`
}

type Adapter struct{}

func (Adapter) Name() string { return "warp" }

func (Adapter) Path() (string, error) {
	var configPath string
	
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, ".warp", "mcp_config.json")
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, ".config", "warp", "mcp_config.json")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configPath = filepath.Join(appData, "warp", "mcp_config.json")
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	
	return configPath, nil
}

func (a Adapter) Detect() (bool, error) {
	p, err := a.Path()
	if err != nil {
		return false, err
	}
	
	// Check if Warp config directory exists
	configDir := filepath.Dir(p)
	if _, err := os.Stat(configDir); err != nil {
		if os.IsNotExist(err) {
			// Also check for Warp app installation on macOS
			if runtime.GOOS == "darwin" {
				if _, err := os.Stat("/Applications/Warp.app"); err == nil {
					return true, nil
				}
			}
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}

func (a Adapter) Load() (*WarpConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &WarpConfig{MCPServers: map[string]WarpServer{}}, nil
		}
		return nil, err
	}
	
	var c WarpConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return &WarpConfig{MCPServers: map[string]WarpServer{}}, nil
	}
	
	if c.MCPServers == nil {
		c.MCPServers = map[string]WarpServer{}
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

// Apply merges canonical servers into Warp config, preserving unmanaged entries.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	cc, err := a.Load()
	if err != nil {
		return "", err
	}
	
	before, _ := json.MarshalIndent(cc, "", "  ")

	// Build desired set from canonical enabled servers
	enabled := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled {
			enabled[s.Name] = s
		}
	}

	// Start with existing; remove entries not in enabled
	newConfig := WarpConfig{MCPServers: map[string]WarpServer{}}
	for name, srv := range cc.MCPServers {
		if _, keep := enabled[name]; keep {
			newConfig.MCPServers[name] = srv
		}
	}
	
	// Add/update enabled ones from canonical
	for name, s := range enabled {
		newConfig.MCPServers[name] = WarpServer{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
			Enabled: true, // Always enabled in Warp when present
		}
	}

	after, _ := json.MarshalIndent(newConfig, "", "  ")
	diffStr := diff.GenerateColorDiff(string(before), string(after))

	// Write the new configuration
	if _, err := a.Backup(); err != nil {
		return diffStr, err
	}
	
	p, err := a.Path()
	if err != nil {
		return diffStr, err
	}
	
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return diffStr, err
	}
	
	if err := os.WriteFile(p, after, 0o644); err != nil {
		return diffStr, err
	}
	
	return diffStr, nil
}