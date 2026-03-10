package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ResistanceIsUseless/mseep/internal/app"
	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/style"
	"github.com/spf13/cobra"
)

func cmdServers() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "servers",
		Aliases: []string{"server", "srv"},
		Short:   "Manage MCP servers",
		Long:    "Add, list, show, and remove MCP servers from the canonical configuration.",
	}

	cmd.AddCommand(cmdServerAdd(), cmdServerList(), cmdServerShow(), cmdServerRemove())
	return cmd
}

func cmdServerAdd() *cobra.Command {
	var (
		command       string
		args          []string
		env           []string
		tags          []string
		transport     string
		description   string
		repository    string
		prerequisites []string
		setupSteps    []string
		enabled       bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new MCP server",
		Long: `Add a new MCP server to the canonical configuration.

Examples:
  # Simple stdio server
  mseep servers add my-server --command npx --args @modelcontextprotocol/server-filesystem,/path

  # Server with environment variables
  mseep servers add github --command npx --args @modelcontextprotocol/server-github \
    --env GITHUB_TOKEN=ghp_xxx

  # Server with setup instructions
  mseep servers add ghidra-mcp --command python --args bridge_mcp_ghidra.py \
    --description "Ghidra reverse engineering integration" \
    --repository https://github.com/bethington/ghidra-mcp \
    --prereq "ghidra" --prereq "python3" --prereq "pip" \
    --setup "Download GhidraMCP-4.3.0.zip from releases" \
    --setup "In Ghidra: File > Install Extensions > Add and select the ZIP" \
    --setup "Restart Ghidra, enable plugin: File > Configure > Configure All Plugins > GhidraMCP" \
    --setup "Start server: Tools > GhidraMCP > Start MCP Server" \
    --setup "Install Python bridge: pip install -r requirements.txt"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, cmdArgs []string) error {
			name := cmdArgs[0]

			// Parse environment variables
			envMap := make(map[string]string)
			for _, e := range env {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					envMap[parts[0]] = parts[1]
				} else {
					envMap[parts[0]] = "" // Placeholder for user to fill
				}
			}

			server := config.Server{
				Name:          name,
				Command:       command,
				Args:          args,
				Env:           envMap,
				Tags:          tags,
				Transport:     transport,
				Description:   description,
				Repository:    repository,
				Prerequisites: prerequisites,
				SetupSteps:    setupSteps,
				Enabled:       enabled,
			}

			return runServerAdd(server)
		},
	}

	cmd.Flags().StringVar(&command, "command", "", "Command to run (e.g., npx, python, node)")
	cmd.Flags().StringSliceVar(&args, "args", nil, "Command arguments (comma-separated or multiple flags)")
	cmd.Flags().StringSliceVar(&env, "env", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for categorization")
	cmd.Flags().StringVar(&transport, "transport", "stdio", "Transport type: stdio, http, tcp")
	cmd.Flags().StringVar(&description, "description", "", "Description of what this server does")
	cmd.Flags().StringVar(&repository, "repository", "", "Repository URL (GitHub, GitLab, etc.)")
	cmd.Flags().StringSliceVar(&prerequisites, "prereq", nil, "Prerequisites (e.g., python3, ghidra)")
	cmd.Flags().StringSliceVar(&setupSteps, "setup", nil, "Setup instructions (use multiple --setup flags)")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Enable server immediately after adding")

	cmd.MarkFlagRequired("command")

	return cmd
}

func cmdServerList() *cobra.Command {
	var jsonOut bool
	var showDisabled bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServerList(jsonOut, showDisabled)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&showDisabled, "all", "a", true, "Show disabled servers too")

	return cmd
}

func cmdServerShow() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show detailed information about a server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServerShow(args[0], jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func cmdServerRemove() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a server from the configuration",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServerRemove(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

// Implementation functions

func runServerAdd(server config.Server) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}

	// Check if server already exists
	if existing := a.Canon.FindByName(server.Name); existing != nil {
		return fmt.Errorf("server %q already exists", server.Name)
	}

	// Validate command is provided
	if server.Command == "" {
		return fmt.Errorf("--command is required")
	}

	// Add server to canonical config
	a.Canon.Servers = append(a.Canon.Servers, server)

	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(style.Success(fmt.Sprintf("Added server %q", server.Name)))

	// Show setup instructions if present
	if len(server.SetupSteps) > 0 {
		fmt.Println()
		fmt.Println(style.Header("Setup Instructions:"))
		for i, step := range server.SetupSteps {
			fmt.Printf("  %d. %s\n", i+1, step)
		}
	}

	if len(server.Prerequisites) > 0 {
		fmt.Println()
		fmt.Printf("ℹ Prerequisites: %s\n", strings.Join(server.Prerequisites, ", "))
	}

	if !server.Enabled {
		fmt.Println()
		fmt.Printf("ℹ Server added but disabled. Run: mseep enable %s\n", server.Name)
	}

	return nil
}

func runServerList(jsonOut, showDisabled bool) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}

	if jsonOut {
		out, err := json.MarshalIndent(a.Canon.Servers, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	if len(a.Canon.Servers) == 0 {
		fmt.Println("No servers configured.")
		fmt.Println("Add one with: mseep servers add <name> --command <cmd>")
		return nil
	}

	fmt.Println(style.Header("MCP Servers"))
	fmt.Println()

	for _, s := range a.Canon.Servers {
		if !showDisabled && !s.Enabled {
			continue
		}

		status := "○"
		if s.Enabled {
			status = "●"
		}

		name := s.Name
		if s.Description != "" {
			name = fmt.Sprintf("%s - %s", s.Name, truncate(s.Description, 50))
		}

		fmt.Printf("  %s %s\n", status, name)

		// Show tags if present
		if len(s.Tags) > 0 {
			fmt.Printf("      Tags: %s\n", strings.Join(s.Tags, ", "))
		}
	}

	return nil
}

func runServerShow(name string, jsonOut bool) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}

	server := a.Canon.FindByName(name)
	if server == nil {
		return fmt.Errorf("server %q not found", name)
	}

	if jsonOut {
		out, err := json.MarshalIndent(server, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	}

	// Pretty print server details
	status := "Disabled"
	if server.Enabled {
		status = "Enabled"
	}

	fmt.Println(style.Header(server.Name))
	fmt.Printf("  Status:    %s\n", status)
	fmt.Printf("  Command:   %s %s\n", server.Command, strings.Join(server.Args, " "))
	if server.Transport != "" {
		fmt.Printf("  Transport: %s\n", server.Transport)
	}

	if server.Description != "" {
		fmt.Printf("  Description: %s\n", server.Description)
	}

	if server.Repository != "" {
		fmt.Printf("  Repository:  %s\n", server.Repository)
	}

	if len(server.Tags) > 0 {
		fmt.Printf("  Tags: %s\n", strings.Join(server.Tags, ", "))
	}

	if len(server.Aliases) > 0 {
		fmt.Printf("  Aliases: %s\n", strings.Join(server.Aliases, ", "))
	}

	if len(server.Env) > 0 {
		fmt.Println("  Environment:")
		for k, v := range server.Env {
			display := v
			if display == "" {
				display = style.Warning("<not set>")
			} else if len(display) > 20 {
				display = display[:17] + "..."
			}
			fmt.Printf("    %s=%s\n", k, display)
		}
	}

	if len(server.Prerequisites) > 0 {
		fmt.Println()
		fmt.Println(style.Header("Prerequisites:"))
		for _, p := range server.Prerequisites {
			fmt.Printf("  • %s\n", p)
		}
	}

	if len(server.SetupSteps) > 0 {
		fmt.Println()
		fmt.Println(style.Header("Setup Instructions:"))
		for i, step := range server.SetupSteps {
			fmt.Printf("  %d. %s\n", i+1, step)
		}
	}

	if len(server.Secrets) > 0 {
		fmt.Println()
		fmt.Println(style.Header("Required Secrets:"))
		for _, sec := range server.Secrets {
			req := ""
			if sec.Required {
				req = " (required)"
			}
			fmt.Printf("  • %s%s\n", sec.Name, req)
			if sec.Description != "" {
				fmt.Printf("    %s\n", sec.Description)
			}
			if sec.URL != "" {
				fmt.Printf("    Get it: %s\n", sec.URL)
			}
		}
	}

	return nil
}

func runServerRemove(name string, force bool) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}

	// Find server index
	idx := -1
	for i, s := range a.Canon.Servers {
		if s.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("server %q not found", name)
	}

	if !force {
		fmt.Printf("Remove server %q? [y/N] ", name)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Remove server
	a.Canon.Servers = append(a.Canon.Servers[:idx], a.Canon.Servers[idx+1:]...)

	// Also remove from profiles
	for profileName, servers := range a.Canon.Profiles {
		filtered := make([]string, 0, len(servers))
		for _, s := range servers {
			if s != name {
				filtered = append(filtered, s)
			}
		}
		a.Canon.Profiles[profileName] = filtered
	}

	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(style.Success(fmt.Sprintf("Removed server %q", name)))
	return nil
}

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
