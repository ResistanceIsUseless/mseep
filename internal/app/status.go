package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mseep/internal/adapters/claude"
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

	// TODO: Add Cursor and Cline adapters when implemented
	if client == "cursor" {
		report.Clients = append(report.Clients, ClientStatus{
			Name:      "cursor",
			Installed: false,
			Servers:   []ServerStatus{},
		})
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

	// Human-readable format
	var output strings.Builder

	for _, clientStatus := range report.Clients {
		output.WriteString(fmt.Sprintf("Client: %s\n", clientStatus.Name))
		
		if !clientStatus.Installed {
			output.WriteString("  Status: Not installed\n\n")
			continue
		}

		output.WriteString(fmt.Sprintf("  Status: Installed\n"))
		output.WriteString(fmt.Sprintf("  Config: %s\n\n", clientStatus.Path))

		if len(clientStatus.Servers) == 0 {
			output.WriteString("  No servers configured\n\n")
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

		// Display synced servers
		if len(synced) > 0 {
			output.WriteString("  ✓ Enabled (synced):\n")
			for _, srv := range synced {
				tags := ""
				if len(srv.Tags) > 0 {
					tags = fmt.Sprintf(" [%s]", strings.Join(srv.Tags, ", "))
				}
				output.WriteString(fmt.Sprintf("    - %s%s\n", srv.Name, tags))
			}
			output.WriteString("\n")
		}

		// Display servers only in canonical
		if len(canonOnly) > 0 {
			output.WriteString("  ⚠ Enabled in canonical only (run 'mseep apply' to sync):\n")
			for _, srv := range canonOnly {
				tags := ""
				if len(srv.Tags) > 0 {
					tags = fmt.Sprintf(" [%s]", strings.Join(srv.Tags, ", "))
				}
				output.WriteString(fmt.Sprintf("    - %s%s\n", srv.Name, tags))
			}
			output.WriteString("\n")
		}

		// Display servers only in client
		if len(clientOnly) > 0 {
			output.WriteString("  ⚠ Enabled in client only (not managed by mseep):\n")
			for _, srv := range clientOnly {
				output.WriteString(fmt.Sprintf("    - %s\n", srv.Name))
			}
			output.WriteString("\n")
		}

		// Summary
		total := len(clientStatus.Servers)
		syncedCount := len(synced)
		output.WriteString(fmt.Sprintf("  Summary: %d servers total, %d synced, %d out of sync\n\n", 
			total, syncedCount, total-syncedCount))
	}

	return output.String(), nil
}