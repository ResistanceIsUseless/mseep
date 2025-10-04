package vscode

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

// VS Code MCP config shape
// Path varies by platform:
// - macOS: ~/Library/Application Support/Code/User/settings.json
// - Linux: ~/.config/Code/User/settings.json  
// - Windows: %APPDATA%\Code\User\settings.json

type VSCodeConfig struct {
	MCPServers map[string]VSCodeServer `json:"mcp.servers,omitempty"`
	// Other VS Code settings preserved
	Other map[string]interface{} `json:"-"`
}

type VSCodeServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Adapter struct{}

func (Adapter) Name() string { return "vscode" }

func (Adapter) Path() (string, error) {
	var configPath string
	
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, ".config", "Code", "User", "settings.json")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		configPath = filepath.Join(appData, "Code", "User", "settings.json")
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
	_, err = os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (a Adapter) Load() (*VSCodeConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &VSCodeConfig{MCPServers: map[string]VSCodeServer{}}, nil
		}
		return nil, err
	}
	
	// Parse the full settings.json to preserve other settings
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(b, &rawConfig); err != nil {
		return nil, err
	}
	
	var c VSCodeConfig
	c.Other = make(map[string]interface{})
	
	// Extract MCP servers if they exist
	if mcpServers, exists := rawConfig["mcp.servers"]; exists {
		if servers, ok := mcpServers.(map[string]interface{}); ok {
			c.MCPServers = make(map[string]VSCodeServer)
			for name, serverData := range servers {
				if serverMap, ok := serverData.(map[string]interface{}); ok {
					server := VSCodeServer{}
					
					if cmd, ok := serverMap["command"].(string); ok {
						server.Command = cmd
					}
					
					if args, ok := serverMap["args"].([]interface{}); ok {
						server.Args = make([]string, len(args))
						for i, arg := range args {
							if argStr, ok := arg.(string); ok {
								server.Args[i] = argStr
							}
						}
					}
					
					if env, ok := serverMap["env"].(map[string]interface{}); ok {
						server.Env = make(map[string]string)
						for key, val := range env {
							if valStr, ok := val.(string); ok {
								server.Env[key] = valStr
							}
						}
					}
					
					c.MCPServers[name] = server
				}
			}
		}
	}
	
	if c.MCPServers == nil {
		c.MCPServers = map[string]VSCodeServer{}
	}
	
	// Preserve all other settings
	for key, value := range rawConfig {
		if key != "mcp.servers" {
			c.Other[key] = value
		}
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

// Apply merges canonical servers into VS Code config, preserving unmanaged entries and other settings.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	cc, err := a.Load()
	if err != nil {
		return "", err
	}
	
	// Create the full config map for before/after comparison
	beforeMap := make(map[string]interface{})
	for key, value := range cc.Other {
		beforeMap[key] = value
	}
	if len(cc.MCPServers) > 0 {
		beforeMap["mcp.servers"] = cc.MCPServers
	}
	before, _ := json.MarshalIndent(beforeMap, "", "  ")

	// Build desired set from canonical enabled servers
	enabled := map[string]config.Server{}
	for _, s := range canon.Servers {
		if s.Enabled {
			enabled[s.Name] = s
		}
	}

	// Start with existing; remove entries not in enabled
	newServers := map[string]VSCodeServer{}
	for name, srv := range cc.MCPServers {
		if _, keep := enabled[name]; keep {
			newServers[name] = srv
		}
	}
	
	// Add/update enabled ones from canonical
	for name, s := range enabled {
		newServers[name] = VSCodeServer{
			Command: s.Command,
			Args:    s.Args,
			Env:     s.Env,
		}
	}

	// Create the full config map for after comparison
	afterMap := make(map[string]interface{})
	for key, value := range cc.Other {
		afterMap[key] = value
	}
	if len(newServers) > 0 {
		afterMap["mcp.servers"] = newServers
	}
	after, _ := json.MarshalIndent(afterMap, "", "  ")
	
	diffStr := diff.GenerateColorDiff(string(before), string(after))

	// Write the complete settings.json back
	if _, err := a.Backup(); err != nil {
		return diffStr, err
	}
	
	p, err := a.Path()
	if err != nil {
		return diffStr, err
	}
	
	if err := os.WriteFile(p, after, 0o644); err != nil {
		return diffStr, err
	}
	
	return diffStr, nil
}