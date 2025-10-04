package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mseep/internal/config"
	"mseep/internal/style"
)

// ListProfiles returns a formatted list of profiles
func (a *App) ListProfiles(jsonOutput bool) (string, error) {
	if jsonOutput {
		output, err := json.MarshalIndent(a.Canon.Profiles, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error formatting json: %w", err)
		}
		return string(output), nil
	}

	if len(a.Canon.Profiles) == 0 {
		return style.Muted("No profiles configured") + "\n", nil
	}

	var output strings.Builder
	output.WriteString(style.Title("Profiles"))
	output.WriteString("\n")

	// Sort profile names
	names := make([]string, 0, len(a.Canon.Profiles))
	for name := range a.Canon.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		servers := a.Canon.Profiles[name]
		output.WriteString("\n")
		output.WriteString(style.Header(name))
		
		if len(servers) == 0 {
			output.WriteString(style.Muted("  (no servers)") + "\n")
		} else {
			output.WriteString(style.Muted(fmt.Sprintf("  %d servers:", len(servers))) + "\n")
			for _, serverName := range servers {
				// Check if server exists
				exists := false
				for _, srv := range a.Canon.Servers {
					if srv.Name == serverName {
						exists = true
						break
					}
				}
				
				if exists {
					output.WriteString(style.ListItem(serverName) + "\n")
				} else {
					output.WriteString(style.Warning(fmt.Sprintf("• %s (server not found)", serverName)) + "\n")
				}
			}
		}
	}

	output.WriteString("\n" + style.Muted(fmt.Sprintf("Total: %d profiles", len(a.Canon.Profiles))) + "\n")
	return output.String(), nil
}

// CreateProfile creates a new profile
func (a *App) CreateProfile(name string, serverNames []string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Initialize profiles map if nil
	if a.Canon.Profiles == nil {
		a.Canon.Profiles = make(map[string][]string)
	}

	// Check if profile already exists
	if _, exists := a.Canon.Profiles[name]; exists {
		return fmt.Errorf("profile %q already exists", name)
	}

	// Validate that all servers exist
	for _, serverName := range serverNames {
		found := false
		for _, srv := range a.Canon.Servers {
			if srv.Name == serverName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("server %q not found", serverName)
		}
	}

	// Create the profile
	a.Canon.Profiles[name] = serverNames

	// Save configuration
	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// DeleteProfile removes a profile
func (a *App) DeleteProfile(name string) error {
	if a.Canon.Profiles == nil {
		return fmt.Errorf("no profiles configured")
	}

	if _, exists := a.Canon.Profiles[name]; !exists {
		return fmt.Errorf("profile %q not found", name)
	}

	delete(a.Canon.Profiles, name)

	// Save configuration
	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// UpdateProfile updates an existing profile
func (a *App) UpdateProfile(name string, serverNames []string) error {
	if a.Canon.Profiles == nil {
		a.Canon.Profiles = make(map[string][]string)
	}

	if _, exists := a.Canon.Profiles[name]; !exists {
		return fmt.Errorf("profile %q not found", name)
	}

	// Validate that all servers exist
	for _, serverName := range serverNames {
		found := false
		for _, srv := range a.Canon.Servers {
			if srv.Name == serverName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("server %q not found", serverName)
		}
	}

	// Update the profile
	a.Canon.Profiles[name] = serverNames

	// Save configuration
	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// CreateProfileFromCurrent creates a profile from currently enabled servers
func (a *App) CreateProfileFromCurrent(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Initialize profiles map if nil
	if a.Canon.Profiles == nil {
		a.Canon.Profiles = make(map[string][]string)
	}

	// Check if profile already exists
	if _, exists := a.Canon.Profiles[name]; exists {
		return fmt.Errorf("profile %q already exists", name)
	}

	// Collect enabled servers
	var enabledServers []string
	for _, srv := range a.Canon.Servers {
		if srv.Enabled {
			enabledServers = append(enabledServers, srv.Name)
		}
	}

	if len(enabledServers) == 0 {
		return fmt.Errorf("no servers are currently enabled")
	}

	// Create the profile
	a.Canon.Profiles[name] = enabledServers

	// Save configuration
	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}