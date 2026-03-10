package warp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/diff"
)

// Warp MCP config shape
// Warp stores MCP configuration in:
// 1. Legacy JSON file: ~/.warp/mcp_config.json
// 2. SQLite database (newer): ~/Library/Group Containers/2BBY89MBSN.dev.warp/Library/Application Support/dev.warp.Warp-Stable/warp.sqlite

type WarpConfig struct {
	MCPServers map[string]WarpServer `json:"mcp_servers"`
}

type WarpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Enabled bool              `json:"enabled,omitempty"`
}

// WarpDBServer represents the JSON structure stored in Warp's SQLite database
type WarpDBServer struct {
	Name          string `json:"name"`
	UUID          string `json:"uuid"`
	TransportType struct {
		CLIServer *struct {
			Command       string   `json:"command"`
			Args          []string `json:"args"`
			CwdParameter  *string  `json:"cwd_parameter"`
			StaticEnvVars []struct {
				Name string `json:"name"`
			} `json:"static_env_vars"`
		} `json:"CLIServer,omitempty"`
		SSEServer *struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers,omitempty"`
		} `json:"SSEServer,omitempty"`
	} `json:"transport_type"`
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

// dbPath returns the path to Warp's SQLite database (macOS only)
func (Adapter) dbPath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("SQLite database only available on macOS")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "Group Containers", "2BBY89MBSN.dev.warp",
		"Library", "Application Support", "dev.warp.Warp-Stable", "warp.sqlite"), nil
}

func (a Adapter) Detect() (bool, error) {
	// Check for SQLite database first (newer Warp)
	if dbPath, err := a.dbPath(); err == nil {
		if _, err := os.Stat(dbPath); err == nil {
			return true, nil
		}
	}

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

// LoadFromDB loads MCP servers from Warp's SQLite database
func (a Adapter) LoadFromDB() (*WarpConfig, error) {
	dbPath, err := a.dbPath()
	if err != nil {
		return nil, err
	}

	// Open database in read-only mode with immutable flag to avoid locking issues
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro&immutable=1", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open Warp database: %w", err)
	}
	defer db.Close()

	// Query MCP server configurations from generic_string_objects
	rows, err := db.Query("SELECT data FROM generic_string_objects WHERE data LIKE '%transport_type%'")
	if err != nil {
		return nil, fmt.Errorf("failed to query MCP servers: %w", err)
	}
	defer rows.Close()

	wc := &WarpConfig{MCPServers: map[string]WarpServer{}}

	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}

		var srv WarpDBServer
		if err := json.Unmarshal([]byte(data), &srv); err != nil {
			continue
		}

		// Convert to WarpServer format
		if srv.TransportType.CLIServer != nil {
			cli := srv.TransportType.CLIServer
			ws := WarpServer{
				Command: cli.Command,
				Args:    cli.Args,
				Enabled: true,
			}

			// Convert static_env_vars to env map (values would need to be fetched separately)
			if len(cli.StaticEnvVars) > 0 {
				ws.Env = make(map[string]string)
				for _, ev := range cli.StaticEnvVars {
					ws.Env[ev.Name] = "" // Placeholder - actual values stored elsewhere
				}
			}

			wc.MCPServers[srv.Name] = ws
		}
	}

	return wc, nil
}

func (a Adapter) Load() (*WarpConfig, error) {
	// Try loading from SQLite database first (newer Warp versions)
	if dbConfig, err := a.LoadFromDB(); err == nil && len(dbConfig.MCPServers) > 0 {
		return dbConfig, nil
	}

	// Fall back to legacy JSON file
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
// Note: This only writes to the legacy JSON file. Warp's SQLite database is read-only
// from our perspective - users should configure servers through Warp's UI for the DB.
func (a Adapter) Apply(canon *config.Canonical) (string, error) {
	// Load from legacy JSON file for Apply (we can't write to SQLite)
	p, err := a.Path()
	if err != nil {
		return "", err
	}

	var cc *WarpConfig
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			cc = &WarpConfig{MCPServers: map[string]WarpServer{}}
		} else {
			return "", err
		}
	} else {
		if err := json.Unmarshal(b, &cc); err != nil {
			cc = &WarpConfig{MCPServers: map[string]WarpServer{}}
		}
		if cc.MCPServers == nil {
			cc.MCPServers = map[string]WarpServer{}
		}
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

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return diffStr, err
	}

	if err := os.WriteFile(p, after, 0o644); err != nil {
		return diffStr, err
	}

	return diffStr, nil
}
