package cline

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

// Cline MCP config shape
// Cline stores its MCP configuration in its own extension settings
// Path varies by platform (similar to VS Code but in Cline extension data):
// - macOS: ~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/mcp_servers.json
// - Linux: ~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/mcp_servers.json
// - Windows: %APPDATA%\Code\User\globalStorage\saoudrizwan.claude-dev\mcp_servers.json

type ClineConfig struct {
	MCPServers map[string]ClineServer `json:"mcpServers"`
}

type ClineServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Adapter struct{}

func (Adapter) Name() string { return "cline" }

func (Adapter) Path() (string, error) {
	var configPath string
	
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "mcp_servers.json")
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, ".config", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "mcp_servers.json")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configPath = filepath.Join(appData, "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "mcp_servers.json")
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
	
	// Check if the parent directory exists (indicating Cline extension is installed)
	parentDir := filepath.Dir(p)
	if _, err := os.Stat(parentDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	
	// Config file might not exist yet, but extension directory does
	return true, nil
}

func (a Adapter) Load() (*ClineConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// Create empty config if file doesn't exist
			return &ClineConfig{MCPServers: map[string]ClineServer{}}, nil
		}
		return nil, err
	}
	
	var c ClineConfig
	if err := json.Unmarshal(b, &c); err != nil {
		// If JSON is invalid, start with empty config
		return &ClineConfig{MCPServers: map[string]ClineServer{}}, nil
	}
	
	if c.MCPServers == nil {
		c.MCPServers = map[string]ClineServer{}
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

// Apply merges canonical servers into Cline config, preserving unmanaged entries.
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
	newConfig := ClineConfig{MCPServers: map[string]ClineServer{}}
	for name, srv := range cc.MCPServers {
		if _, keep := enabled[name]; keep {
			newConfig.MCPServers[name] = srv
		}
	}
	
	// Add/update enabled ones from canonical
	for name, s := range enabled {
		newConfig.MCPServers[name] = ClineServer{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
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