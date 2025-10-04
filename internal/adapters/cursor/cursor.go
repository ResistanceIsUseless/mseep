package cursor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mseep/internal/config"
	"mseep/internal/diff"
)

// Cursor MCP config shape
// Path: ~/Library/Application Support/Cursor/User/settings.json (macOS)
// Path: ~/.config/Cursor/User/settings.json (Linux)
// Path: %APPDATA%\Cursor\User\settings.json (Windows)

type CursorConfig struct {
	MCPServers map[string]CursorServer `json:"mcp.servers,omitempty"`
	// Other VS Code settings would be here, but we only care about MCP
	Other map[string]interface{} `json:"-"` // Preserve other settings
}

type CursorServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Adapter struct{}

func (Adapter) Name() string { return "cursor" }

func (Adapter) Path() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// macOS path
	return filepath.Join(h, "Library", "Application Support", "Cursor", "User", "settings.json"), nil
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

func (a Adapter) Load() (*CursorConfig, error) {
	p, err := a.Path()
	if err != nil {
		return nil, err
	}
	
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &CursorConfig{MCPServers: map[string]CursorServer{}}, nil
		}
		return nil, err
	}
	
	// Parse the full settings.json to preserve other settings
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(b, &rawConfig); err != nil {
		return nil, err
	}
	
	var c CursorConfig
	c.Other = make(map[string]interface{})
	
	// Extract MCP servers if they exist
	if mcpServers, exists := rawConfig["mcp.servers"]; exists {
		if servers, ok := mcpServers.(map[string]interface{}); ok {
			c.MCPServers = make(map[string]CursorServer)
			for name, serverData := range servers {
				if serverMap, ok := serverData.(map[string]interface{}); ok {
					server := CursorServer{}
					
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
		c.MCPServers = map[string]CursorServer{}
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

// Apply merges canonical servers into Cursor config, preserving unmanaged entries and other settings.
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
	newServers := map[string]CursorServer{}
	for name, srv := range cc.MCPServers {
		if _, keep := enabled[name]; keep {
			newServers[name] = srv
		}
	}
	
	// Add/update enabled ones from canonical
	for name, s := range enabled {
		newServers[name] = CursorServer{
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