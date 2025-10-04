package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.0.1"
)

func main() {
	root := &cobra.Command{
		Use:   "mseep",
		Short: "mseep: MCP Server Enable/Disable & Profiles (TUI + CLI)",
		Long:  "mseep is a fast TUI/CLI to manage MCP servers across clients (Claude, Cursor, etc.).",
	}

	root.AddCommand(cmdTUI(), cmdEnable(), cmdDisable(), cmdToggle(), cmdStatus(), cmdHealth(), cmdApply(), cmdProfiles())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cmdTUI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}
	return cmd
}

func cmdEnable() *cobra.Command {
	var client, yes string
	cmd := &cobra.Command{
		Use:   "enable <query>",
		Short: "Enable server(s) by fuzzy query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			return cmdEnableDisableToggle("enable", query, client, yes == "true")
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client (e.g., claude, cursor)")
	cmd.Flags().StringVar(&yes, "yes", "false", "Assume yes; no prompt if ambiguous")
	return cmd
}

func cmdDisable() *cobra.Command {
	var client, yes string
	cmd := &cobra.Command{
		Use:   "disable <query>",
		Short: "Disable server(s) by fuzzy query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			return cmdEnableDisableToggle("disable", query, client, yes == "true")
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client (e.g., claude, cursor)")
	cmd.Flags().StringVar(&yes, "yes", "false", "Assume yes; no prompt if ambiguous")
	return cmd
}

func cmdToggle() *cobra.Command {
	var client, yes string
	cmd := &cobra.Command{
		Use:   "toggle <query>",
		Short: "Toggle server(s) by fuzzy query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			return cmdEnableDisableToggle("toggle", query, client, yes == "true")
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client (e.g., claude, cursor)")
	cmd.Flags().StringVar(&yes, "yes", "false", "Assume yes; no prompt if ambiguous")
	return cmd
}

func cmdStatus() *cobra.Command {
	var client string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of clients and servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(client, jsonOut)
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client (empty=all)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output JSON")
	return cmd
}

func cmdHealth() *cobra.Command {
	var client, server string
	var fix bool
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Run health checks (manual, opt-in)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHealth(client, server, fix)
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client (empty=all)")
	cmd.Flags().StringVar(&server, "server", "", "Limit to one server by query")
	cmd.Flags().BoolVar(&fix, "fix", false, "Auto-disable failing servers")
	return cmd
}

func cmdApply() *cobra.Command {
	var client, profile string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply canonical state to client configs (with diff preview)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(client, profile)
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "Target client (empty=all)")
	cmd.Flags().StringVar(&profile, "profile", "", "Profile to apply before writing")
	return cmd
}

func cmdProfiles() *cobra.Command {
	var jsonOut bool
	
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage server profiles",
		Long:  "List, create, delete, and manage server profiles",
	}

	// List profiles subcommand
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfilesList(jsonOut)
		},
	}
	listCmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	// Create profile subcommand
	createCmd := &cobra.Command{
		Use:   "create <name> [server1 server2 ...]",
		Short: "Create a new profile",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			servers := args[1:]
			return runProfilesCreate(name, servers)
		},
	}

	// Create profile from current state
	var fromCurrent bool
	saveCmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save current enabled servers as a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfilesSave(args[0])
		},
	}
	saveCmd.Flags().BoolVar(&fromCurrent, "from-current", true, "Create from currently enabled servers")

	// Delete profile subcommand
	deleteCmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfilesDelete(args[0])
		},
	}

	// Apply profile subcommand
	applyCmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Apply a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfilesApply(args[0])
		},
	}

	cmd.AddCommand(listCmd, createCmd, saveCmd, deleteCmd, applyCmd)
	return cmd
}
