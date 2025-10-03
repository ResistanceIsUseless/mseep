package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mseep/internal/adapters/claude"
	"mseep/internal/config"
	"mseep/internal/diff"
)

// Apply applies the canonical configuration to the specified client
func (a *App) Apply(client, profile string, autoApprove bool) error {
	// Apply profile if specified
	if profile != "" {
		if err := a.applyProfile(profile); err != nil {
			return fmt.Errorf("failed to apply profile %q: %w", profile, err)
		}
	}

	// Determine which clients to apply to
	clients := []string{}
	if client == "" || client == "all" {
		// Apply to all detected clients
		if ca := (claude.Adapter{}); detectClient(ca) {
			clients = append(clients, "claude")
		}
		// TODO: Add Cursor and Cline when implemented
	} else {
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return fmt.Errorf("no clients detected or specified")
	}

	// Apply to each client
	for _, c := range clients {
		fmt.Printf("\nApplying configuration to %s...\n", c)
		
		switch c {
		case "claude":
			if err := a.applyToClaude(autoApprove); err != nil {
				return fmt.Errorf("failed to apply to claude: %w", err)
			}
		case "cursor":
			fmt.Println("Cursor support not yet implemented")
		case "cline":
			fmt.Println("Cline support not yet implemented")
		default:
			return fmt.Errorf("unknown client: %s", c)
		}
	}

	return nil
}

func (a *App) applyProfile(profileName string) error {
	profile, exists := a.Canon.Profiles[profileName]
	if !exists {
		return fmt.Errorf("profile %q not found", profileName)
	}

	// Reset all servers to disabled
	for i := range a.Canon.Servers {
		a.Canon.Servers[i].Enabled = false
	}

	// Enable servers in the profile
	for _, serverName := range profile {
		found := false
		for i := range a.Canon.Servers {
			if a.Canon.Servers[i].Name == serverName {
				a.Canon.Servers[i].Enabled = true
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Warning: Server %q in profile %q not found in canonical config\n", serverName, profileName)
		}
	}

	// Save the updated canonical config
	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save canonical config: %w", err)
	}

	fmt.Printf("Applied profile %q\n", profileName)
	return nil
}

func (a *App) applyToClaude(autoApprove bool) error {
	ca := claude.Adapter{}
	
	// Check if Claude is installed
	if !detectClient(ca) {
		return fmt.Errorf("Claude Desktop not detected")
	}

	// Load current Claude config
	currentConfig, err := ca.Load()
	if err != nil {
		return fmt.Errorf("failed to load Claude config: %w", err)
	}

	// Build the new configuration
	newConfig := &claude.ClaudeConfig{
		MCPServers: make(map[string]claude.ClaudeServer),
	}

	// First, preserve any unmanaged servers (those not in canonical)
	for name, srv := range currentConfig.MCPServers {
		found := false
		for _, canonSrv := range a.Canon.Servers {
			if canonSrv.Name == name {
				found = true
				break
			}
		}
		if !found {
			// This is an unmanaged server, preserve it
			newConfig.MCPServers[name] = srv
		}
	}

	// Then add enabled servers from canonical
	for _, srv := range a.Canon.Servers {
		if srv.Enabled {
			newConfig.MCPServers[srv.Name] = claude.ClaudeServer{
				Command: srv.Command,
				Args:    srv.Args,
				Env:     srv.Env,
			}
		}
	}

	// Generate diff for preview
	beforeJSON, _ := json.MarshalIndent(currentConfig, "", "  ")
	afterJSON, _ := json.MarshalIndent(newConfig, "", "  ")
	
	if string(beforeJSON) == string(afterJSON) {
		fmt.Println("No changes needed - configuration is already in sync")
		return nil
	}

	// Show diff preview
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("Configuration changes preview:")
	fmt.Println(strings.Repeat("-", 60))
	diffStr := diff.GenerateColorDiff(string(beforeJSON), string(afterJSON))
	fmt.Println(diffStr)
	fmt.Println(strings.Repeat("-", 60))

	// Ask for confirmation unless auto-approve is set
	if !autoApprove {
		fmt.Print("\nApply these changes? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Changes not applied")
			return nil
		}
	}

	// Create backup
	backupPath, err := ca.Backup()
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	if backupPath != "" {
		fmt.Printf("Created backup: %s\n", backupPath)
	}

	// Write the new configuration
	configPath, err := ca.Path()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	if err := os.WriteFile(configPath, afterJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("✓ Configuration applied successfully to Claude Desktop\n")
	return nil
}

func detectClient(adapter interface{ Detect() (bool, error) }) bool {
	detected, _ := adapter.Detect()
	return detected
}