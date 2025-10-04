package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mseep/internal/adapters/claude"
	"mseep/internal/adapters/cline"
	"mseep/internal/adapters/cursor"
	"mseep/internal/adapters/vscode"
	"mseep/internal/adapters/warp"
	"mseep/internal/config"
	"mseep/internal/diff"
	"mseep/internal/style"
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
		adapters := map[string]interface{ Detect() (bool, error) }{
			"claude": claude.Adapter{},
			"cursor": cursor.Adapter{},
			"vscode": vscode.Adapter{},
			"cline":  cline.Adapter{},
			"warp":   warp.Adapter{},
		}
		
		for name, adapter := range adapters {
			if detectClient(adapter) {
				clients = append(clients, name)
			}
		}
	} else {
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return fmt.Errorf("no clients detected or specified")
	}

	// Apply to each client
	for i, c := range clients {
		if len(clients) > 1 {
			fmt.Print(style.ProgressStep(i+1, len(clients), fmt.Sprintf("Applying configuration to %s", c)) + "\n")
		} else {
			fmt.Print(style.Header(fmt.Sprintf("Applying configuration to %s", c)) + "\n")
		}
		
		switch c {
		case "claude":
			if err := a.applyToClaude(autoApprove); err != nil {
				return fmt.Errorf("failed to apply to claude: %w", err)
			}
		case "cursor":
			if err := a.applytoCursor(autoApprove); err != nil {
				return fmt.Errorf("failed to apply to cursor: %w", err)
			}
		case "vscode":
			if err := a.applyToVSCode(autoApprove); err != nil {
				return fmt.Errorf("failed to apply to vscode: %w", err)
			}
		case "cline":
			if err := a.applyToCline(autoApprove); err != nil {
				return fmt.Errorf("failed to apply to cline: %w", err)
			}
		case "warp":
			if err := a.applyToWarp(autoApprove); err != nil {
				return fmt.Errorf("failed to apply to warp: %w", err)
			}
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

	fmt.Print(style.Success(fmt.Sprintf("Applied profile %q", profileName)) + "\n")
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
		fmt.Print(style.Success("No changes needed - configuration is already in sync") + "\n")
		return nil
	}

	// Show diff preview
	fmt.Print("\n" + style.Header("Configuration Changes Preview") + "\n")
	diffStr := diff.GenerateColorDiff(string(beforeJSON), string(afterJSON))
	fmt.Print(style.DiffBox(diffStr) + "\n")

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
			fmt.Print(style.Warning("Changes not applied") + "\n")
			return nil
		}
	}

	// Create backup
	backupPath, err := ca.Backup()
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	if backupPath != "" {
		fmt.Print(style.Muted("Created backup: ") + style.Code(backupPath) + "\n")
	}

	// Write the new configuration
	configPath, err := ca.Path()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	if err := os.WriteFile(configPath, afterJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Print(style.Success("Configuration applied successfully to Claude Desktop") + "\n")
	return nil
}

func (a *App) applytoCursor(autoApprove bool) error {
	ca := cursor.Adapter{}
	
	// Check if Cursor is installed
	if !detectClient(ca) {
		return fmt.Errorf("Cursor not detected")
	}

	// Load current Cursor config
	currentConfig, err := ca.Load()
	if err != nil {
		return fmt.Errorf("failed to load Cursor config: %w", err)
	}

	// Build the new configuration
	newConfig := &cursor.CursorConfig{
		MCPServers: make(map[string]cursor.CursorServer),
		Other:      make(map[string]interface{}),
	}

	// Preserve other settings
	for key, value := range currentConfig.Other {
		newConfig.Other[key] = value
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
			newConfig.MCPServers[srv.Name] = cursor.CursorServer{
				Command: srv.Command,
				Args:    srv.Args,
				Env:     srv.Env,
			}
		}
	}

	// Generate full config maps for diff
	beforeMap := make(map[string]interface{})
	for key, value := range currentConfig.Other {
		beforeMap[key] = value
	}
	if len(currentConfig.MCPServers) > 0 {
		beforeMap["mcp.servers"] = currentConfig.MCPServers
	}

	afterMap := make(map[string]interface{})
	for key, value := range newConfig.Other {
		afterMap[key] = value
	}
	if len(newConfig.MCPServers) > 0 {
		afterMap["mcp.servers"] = newConfig.MCPServers
	}

	beforeJSON, _ := json.MarshalIndent(beforeMap, "", "  ")
	afterJSON, _ := json.MarshalIndent(afterMap, "", "  ")
	
	if string(beforeJSON) == string(afterJSON) {
		fmt.Print(style.Success("No changes needed - configuration is already in sync") + "\n")
		return nil
	}

	// Show diff preview
	fmt.Print("\n" + style.Header("Configuration Changes Preview") + "\n")
	diffStr := diff.GenerateColorDiff(string(beforeJSON), string(afterJSON))
	fmt.Print(style.DiffBox(diffStr) + "\n")

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
			fmt.Print(style.Warning("Changes not applied") + "\n")
			return nil
		}
	}

	// Create backup
	backupPath, err := ca.Backup()
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	if backupPath != "" {
		fmt.Print(style.Muted("Created backup: ") + style.Code(backupPath) + "\n")
	}

	// Write the new configuration
	configPath, err := ca.Path()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	if err := os.WriteFile(configPath, afterJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Print(style.Success("Configuration applied successfully to Cursor") + "\n")
	return nil
}

func (a *App) applyToVSCode(autoApprove bool) error {
	return a.applyToGenericClient(vscode.Adapter{}, "VS Code", autoApprove)
}

func (a *App) applyToCline(autoApprove bool) error {
	return a.applyToGenericClient(cline.Adapter{}, "Cline", autoApprove)
}

func (a *App) applyToWarp(autoApprove bool) error {
	return a.applyToGenericClient(warp.Adapter{}, "Warp", autoApprove)
}

// Generic client application logic to reduce code duplication
func (a *App) applyToGenericClient(adapter interface {
	Name() string
	Detect() (bool, error)
	Apply(*config.Canonical) (string, error)
	Backup() (string, error)
	Path() (string, error)
}, clientName string, autoApprove bool) error {
	
	// Check if client is installed
	if !detectClient(adapter) {
		return fmt.Errorf("%s not detected", clientName)
	}

	// Generate diff preview
	diffStr, err := adapter.Apply(a.Canon)
	if err != nil {
		return fmt.Errorf("failed to apply to %s: %w", clientName, err)
	}

	if diffStr == "" {
		fmt.Print(style.Success("No changes needed - configuration is already in sync") + "\n")
		return nil
	}

	// Show diff preview
	fmt.Print("\n" + style.Header("Configuration Changes Preview") + "\n")
	fmt.Print(style.DiffBox(diffStr) + "\n")

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
			fmt.Print(style.Warning("Changes not applied") + "\n")
			return nil
		}
	}

	// Create backup (already done in Apply, but get path for display)
	configPath, err := adapter.Path()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	fmt.Print(style.Success(fmt.Sprintf("Configuration applied successfully to %s", clientName)) + "\n")
	fmt.Print(style.Muted("Config: ") + style.Code(configPath) + "\n")
	return nil
}

func detectClient(adapter interface{ Detect() (bool, error) }) bool {
	detected, _ := adapter.Detect()
	return detected
}