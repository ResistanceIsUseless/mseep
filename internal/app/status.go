package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mseep/internal/adapters/claude"
	"mseep/internal/adapters/claudecode"
	"mseep/internal/adapters/cline"
	"mseep/internal/adapters/crush"
	"mseep/internal/adapters/cursor"
	"mseep/internal/adapters/opencode"
	"mseep/internal/adapters/vscode"
	"mseep/internal/adapters/warp"
	"mseep/internal/style"
)

type StatusReport struct {
	Clients []ClientStatus `json:"clients"`
}

type ClientStatus struct {
	Name      string         `json:"name"`
	Installed bool           `json:"installed"`
	Path      string         `json:"path,omitempty"`
	Servers   []ServerStatus `json:"servers"`
}

type ServerStatus struct {
	Name            string `json:"name"`
	EnabledCanon    bool   `json:"enabled_canonical"`
	EnabledClient   bool   `json:"enabled_client"`
	InSync          bool   `json:"in_sync"`
	Tags            []string `json:"tags,omitempty"`
	Transport       string `json:"transport,omitempty"`
}

// GetStatusReport returns the raw StatusReport for all (or one) clients.
// Used by the TUI to render the status view without triggering prints.
func (a *App) GetStatusReport(client string) (*StatusReport, error) {
	report := &StatusReport{Clients: []ClientStatus{}}
	adapters := []string{"claude", "claude-code", "cursor", "vscode", "cline", "warp", "crush", "opencode"}
	for _, name := range adapters {
		if client != "" && client != name {
			continue
		}
		cs, err := a.getClientStatusByName(name)
		if err != nil {
			return nil, fmt.Errorf("error getting status for %s: %w", name, err)
		}
		report.Clients = append(report.Clients, cs)
	}
	return report, nil
}

func (a *App) Status(client string, jsonOutput bool) (string, error) {
	report := StatusReport{Clients: []ClientStatus{}}

	// Check each client type
	adapters := []string{"claude", "claude-code", "cursor", "vscode", "cline", "warp", "crush", "opencode"}

	for _, name := range adapters {
		// Skip if specific client requested and this isn't it
		if client != "" && client != name {
			continue
		}

		clientStatus, err := a.getClientStatusByName(name)
		if err != nil {
			return "", fmt.Errorf("error getting status for %s: %w", name, err)
		}

		report.Clients = append(report.Clients, clientStatus)
	}

	// Format output
	if jsonOutput {
		output, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error formatting json: %w", err)
		}
		return string(output), nil
	}

	// Human-readable format with beautiful styling
	var output strings.Builder
	
	output.WriteString(style.Title("mseep Status Report"))
	output.WriteString("\n")

	for i, clientStatus := range report.Clients {
		if i > 0 {
			output.WriteString("\n")
		}
		
		output.WriteString(style.Header(fmt.Sprintf("Client: %s", clientStatus.Name)))
		
		if !clientStatus.Installed {
			output.WriteString(style.Warning("Not installed") + "\n")
			continue
		}

		output.WriteString(style.Success("Installed") + "\n")
		output.WriteString(style.Muted("Config: ") + style.Code(clientStatus.Path) + "\n")

		if len(clientStatus.Servers) == 0 {
			output.WriteString(style.Muted("No servers configured") + "\n")
			continue
		}

		// Group servers by sync status
		var synced, canonOnly, clientOnly []ServerStatus
		for _, srv := range clientStatus.Servers {
			if srv.InSync && srv.EnabledCanon {
				synced = append(synced, srv)
			} else if srv.EnabledCanon && !srv.EnabledClient {
				canonOnly = append(canonOnly, srv)
			} else if !srv.EnabledCanon && srv.EnabledClient {
				clientOnly = append(clientOnly, srv)
			}
		}

		// Create status table
		var tableRows [][]string
		var headers = []string{"Server", "Canonical", "Client", "Status", "Tags"}
		
		for _, srv := range clientStatus.Servers {
			canonStatus := "✗ Disabled"
			if srv.EnabledCanon {
				canonStatus = "✓ Enabled"
			}
			
			clientStatus := "✗ Disabled"
			if srv.EnabledClient {
				clientStatus = "✓ Enabled"
			}
			
			syncStatus := "✗ Out of sync"
			if srv.InSync {
				if srv.EnabledCanon {
					syncStatus = "✓ Synced"
				} else {
					syncStatus = "✓ Synced (disabled)"
				}
			}
			
			tags := strings.Join(srv.Tags, ", ")
			if tags == "" {
				tags = style.Muted("none")
			}
			
			tableRows = append(tableRows, []string{
				srv.Name,
				canonStatus,
				clientStatus,
				syncStatus,
				tags,
			})
		}
		
		output.WriteString("\n")
		output.WriteString(style.StatusTable(tableRows, headers))
		
		// Summary
		total := len(clientStatus.Servers)
		syncedCount := len(synced)
		outOfSyncCount := total - syncedCount
		
		var summaryParts []string
		summaryParts = append(summaryParts, fmt.Sprintf("%d total", total))
		if syncedCount > 0 {
			summaryParts = append(summaryParts, style.Success(fmt.Sprintf("%d synced", syncedCount)))
		}
		if outOfSyncCount > 0 {
			summaryParts = append(summaryParts, style.Warning(fmt.Sprintf("%d out of sync", outOfSyncCount)))
		}
		
		output.WriteString("\n" + style.Muted("Summary: ") + strings.Join(summaryParts, ", ") + "\n")
		
		// Show action hints
		if len(canonOnly) > 0 {
			output.WriteString(style.Muted("💡 Run 'mseep apply' to sync canonical config to client") + "\n")
		}
		if len(clientOnly) > 0 {
			output.WriteString(style.Muted("💡 Unmanaged servers found - consider adding to canonical config") + "\n")
		}
	}

	return output.String(), nil
}

func (a *App) getClientStatusByName(name string) (ClientStatus, error) {
	clientStatus := ClientStatus{
		Name:    name,
		Servers: []ServerStatus{},
	}

	var serverNames []string
	var installed bool
	var path string
	var err error

	// Handle each client type specifically
	switch name {
	case "claude":
		adapter := claude.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCPServers {
				serverNames = append(serverNames, serverName)
			}
		}
	case "cursor":
		adapter := cursor.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCPServers {
				serverNames = append(serverNames, serverName)
			}
		}
	case "vscode":
		adapter := vscode.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCPServers {
				serverNames = append(serverNames, serverName)
			}
		}
	case "cline":
		adapter := cline.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCPServers {
				serverNames = append(serverNames, serverName)
			}
		}
	case "warp":
		adapter := warp.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCPServers {
				serverNames = append(serverNames, serverName)
			}
		}
	case "claude-code":
		adapter := claudecode.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCPServers {
				serverNames = append(serverNames, serverName)
			}
		}
	case "crush":
		adapter := crush.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			// Crush uses "mcp" key; list any server not marked disabled.
			for serverName, srv := range config.MCP {
				if !srv.Disabled {
					serverNames = append(serverNames, serverName)
				}
			}
		}
	case "opencode":
		adapter := opencode.Adapter{}
		installed, err = adapter.Detect()
		if err != nil {
			return clientStatus, err
		}
		if installed {
			path, _ = adapter.Path()
			config, err := adapter.Load()
			if err != nil {
				return clientStatus, err
			}
			for serverName := range config.MCP {
				serverNames = append(serverNames, serverName)
			}
		}
	default:
		return clientStatus, fmt.Errorf("unknown client: %s", name)
	}

	clientStatus.Installed = installed
	clientStatus.Path = path

	if !installed {
		return clientStatus, nil
	}

	// Build server status list
	serverMap := make(map[string]*ServerStatus)

	// First, add all canonical servers
	for _, srv := range a.Canon.Servers {
		serverMap[srv.Name] = &ServerStatus{
			Name:          srv.Name,
			EnabledCanon:  srv.Enabled,
			EnabledClient: false,
			InSync:        false,
			Tags:          srv.Tags,
			Transport:     srv.Transport,
		}
	}

	// Then check which ones are in client config
	for _, serverName := range serverNames {
		if status, exists := serverMap[serverName]; exists {
			status.EnabledClient = true
			status.InSync = (status.EnabledCanon == status.EnabledClient)
		} else {
			// Server in client but not in canonical
			serverMap[serverName] = &ServerStatus{
				Name:          serverName,
				EnabledCanon:  false,
				EnabledClient: true,
				InSync:        false,
			}
		}
	}

	// Convert map to sorted slice
	for _, status := range serverMap {
		// Mark as in sync if both are enabled or both are disabled
		status.InSync = (status.EnabledCanon == status.EnabledClient)
		clientStatus.Servers = append(clientStatus.Servers, *status)
	}
	
	// Sort servers by name
	sort.Slice(clientStatus.Servers, func(i, j int) bool {
		return clientStatus.Servers[i].Name < clientStatus.Servers[j].Name
	})

	return clientStatus, nil
}