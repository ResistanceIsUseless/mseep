package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mseep/internal/adapters/claude"
	"mseep/internal/adapters/cursor"
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

func (a *App) Status(client string, jsonOutput bool) (string, error) {
	report := StatusReport{Clients: []ClientStatus{}}

	// Check Claude if no specific client or client is "claude" 
	if client == "" || client == "claude" {
		claudeAdapter := claude.Adapter{}
		claudeStatus := ClientStatus{
			Name:    "claude",
			Servers: []ServerStatus{},
		}

		// Check if Claude is installed
		installed, err := claudeAdapter.Detect()
		if err != nil {
			return "", fmt.Errorf("error detecting claude: %w", err)
		}
		claudeStatus.Installed = installed

		if installed {
			path, _ := claudeAdapter.Path()
			claudeStatus.Path = path

			// Load Claude config
			claudeConfig, err := claudeAdapter.Load()
			if err != nil {
				return "", fmt.Errorf("error loading claude config: %w", err)
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

			// Then check which ones are in Claude config
			for name := range claudeConfig.MCPServers {
				if status, exists := serverMap[name]; exists {
					status.EnabledClient = true
					status.InSync = (status.EnabledCanon == status.EnabledClient)
				} else {
					// Server in Claude but not in canonical
					serverMap[name] = &ServerStatus{
						Name:          name,
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
				claudeStatus.Servers = append(claudeStatus.Servers, *status)
			}
			
			// Sort servers by name
			sort.Slice(claudeStatus.Servers, func(i, j int) bool {
				return claudeStatus.Servers[i].Name < claudeStatus.Servers[j].Name
			})
		}

		report.Clients = append(report.Clients, claudeStatus)
	}

	// Check Cursor if no specific client or client is "cursor"
	if client == "" || client == "cursor" {
		cursorAdapter := cursor.Adapter{}
		cursorStatus := ClientStatus{
			Name:    "cursor",
			Servers: []ServerStatus{},
		}

		// Check if Cursor is installed
		installed, err := cursorAdapter.Detect()
		if err != nil {
			return "", fmt.Errorf("error detecting cursor: %w", err)
		}
		cursorStatus.Installed = installed

		if installed {
			path, _ := cursorAdapter.Path()
			cursorStatus.Path = path

			// Load Cursor config
			cursorConfig, err := cursorAdapter.Load()
			if err != nil {
				return "", fmt.Errorf("error loading cursor config: %w", err)
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

			// Then check which ones are in Cursor config
			for name := range cursorConfig.MCPServers {
				if status, exists := serverMap[name]; exists {
					status.EnabledClient = true
					status.InSync = (status.EnabledCanon == status.EnabledClient)
				} else {
					// Server in Cursor but not in canonical
					serverMap[name] = &ServerStatus{
						Name:          name,
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
				cursorStatus.Servers = append(cursorStatus.Servers, *status)
			}
			
			// Sort servers by name
			sort.Slice(cursorStatus.Servers, func(i, j int) bool {
				return cursorStatus.Servers[i].Name < cursorStatus.Servers[j].Name
			})
		}

		report.Clients = append(report.Clients, cursorStatus)
	}

	if client == "cline" {
		report.Clients = append(report.Clients, ClientStatus{
			Name:      "cline",
			Installed: false,
			Servers:   []ServerStatus{},
		})
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