package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ResistanceIsUseless/mseep/internal/adapters/claude"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/claudecode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/cline"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/cursor"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/opencode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/vscode"
	"github.com/ResistanceIsUseless/mseep/internal/adapters/warp"
	"github.com/ResistanceIsUseless/mseep/internal/app"
	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/style"
)

// DiscoveredServer holds a server found in a client config
type DiscoveredServer struct {
	Name      string
	Command   string
	Args      []string
	Env       map[string]string
	Transport string
	Source    string // which client it was found in
}

func cmdImport() *cobra.Command {
	var (
		client string
		enable bool
		dryRun bool
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import MCP servers from client configs into canonical",
		Long: `Discover MCP servers configured in your AI clients and import them
into the canonical configuration.

This is useful for:
  - Initial setup: import existing servers from Claude, Cursor, etc.
  - Syncing: ensure all clients have the same servers available
  - Backup: capture server configurations before making changes

Examples:
  # Discover all servers from all clients (dry run)
  mseep import --dry-run

  # Import all discovered servers
  mseep import

  # Import from a specific client
  mseep import --client claude

  # Import and enable immediately
  mseep import --enable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(client, enable, dryRun, force)
		},
	}

	cmd.Flags().StringVar(&client, "client", "", "Import from specific client only")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable imported servers immediately")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be imported without making changes")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing servers with same name")

	return cmd
}

func runImport(clientFilter string, enable, dryRun, force bool) error {
	a, err := app.LoadApp()
	if err != nil {
		return err
	}

	// Discover servers from all clients
	discovered := discoverServers(clientFilter)

	if len(discovered) == 0 {
		fmt.Println("No servers discovered from client configs.")
		return nil
	}

	// Check which are new vs existing
	existing := make(map[string]bool)
	for _, s := range a.Canon.Servers {
		existing[s.Name] = true
	}

	var toImport []DiscoveredServer
	var skipped []DiscoveredServer

	for _, d := range discovered {
		if existing[d.Name] && !force {
			skipped = append(skipped, d)
		} else {
			toImport = append(toImport, d)
		}
	}

	// Display results
	fmt.Println(style.Header("Discovered MCP Servers"))
	fmt.Println()

	if len(toImport) > 0 {
		fmt.Println(style.Success(fmt.Sprintf("  %d servers to import:", len(toImport))))
		for _, d := range toImport {
			status := "new"
			if existing[d.Name] {
				status = "overwrite"
			}
			fmt.Printf("    • %s (%s from %s)\n", d.Name, status, d.Source)
			fmt.Printf("      Command: %s %s\n", d.Command, strings.Join(d.Args, " "))
		}
		fmt.Println()
	}

	if len(skipped) > 0 {
		fmt.Println(style.Warning(fmt.Sprintf("  %d servers skipped (already exist):", len(skipped))))
		for _, d := range skipped {
			fmt.Printf("    • %s (from %s)\n", d.Name, d.Source)
		}
		fmt.Println("    Use --force to overwrite existing servers")
		fmt.Println()
	}

	if dryRun {
		fmt.Println(style.Muted("Dry run - no changes made"))
		return nil
	}

	if len(toImport) == 0 {
		fmt.Println("Nothing to import.")
		return nil
	}

	// Import servers
	for _, d := range toImport {
		// Remove existing if force
		if existing[d.Name] {
			for i, s := range a.Canon.Servers {
				if s.Name == d.Name {
					a.Canon.Servers = append(a.Canon.Servers[:i], a.Canon.Servers[i+1:]...)
					break
				}
			}
		}

		// Add new server
		server := config.Server{
			Name:      d.Name,
			Command:   d.Command,
			Args:      d.Args,
			Env:       d.Env,
			Transport: d.Transport,
			Enabled:   enable,
		}
		a.Canon.Servers = append(a.Canon.Servers, server)
	}

	if err := config.Save("", a.Canon); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(style.Success(fmt.Sprintf("Imported %d servers", len(toImport))))

	if enable {
		fmt.Println("Servers are enabled. Run 'mseep apply' to sync to clients.")
	} else {
		fmt.Println("Servers are disabled. Run 'mseep enable <name>' then 'mseep apply'.")
	}

	return nil
}

func discoverServers(clientFilter string) []DiscoveredServer {
	var discovered []DiscoveredServer
	seen := make(map[string]bool) // dedupe by name

	// Claude
	if clientFilter == "" || clientFilter == "claude" {
		adapter := claude.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCPServers {
					if !seen[name] {
						seen[name] = true
						discovered = append(discovered, DiscoveredServer{
							Name:      name,
							Command:   srv.Command,
							Args:      srv.Args,
							Env:       srv.Env,
							Transport: "stdio",
							Source:    "claude",
						})
					}
				}
			}
		}
	}

	// Claude Code
	if clientFilter == "" || clientFilter == "claude-code" {
		adapter := claudecode.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCPServers {
					if !seen[name] {
						seen[name] = true
						discovered = append(discovered, DiscoveredServer{
							Name:      name,
							Command:   srv.Command,
							Args:      srv.Args,
							Env:       srv.Env,
							Transport: "stdio",
							Source:    "claude-code",
						})
					}
				}
			}
		}
	}

	// Cursor
	if clientFilter == "" || clientFilter == "cursor" {
		adapter := cursor.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCPServers {
					if !seen[name] {
						seen[name] = true
						discovered = append(discovered, DiscoveredServer{
							Name:      name,
							Command:   srv.Command,
							Args:      srv.Args,
							Env:       srv.Env,
							Transport: "stdio",
							Source:    "cursor",
						})
					}
				}
			}
		}
	}

	// VS Code
	if clientFilter == "" || clientFilter == "vscode" {
		adapter := vscode.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCPServers {
					if !seen[name] {
						seen[name] = true
						discovered = append(discovered, DiscoveredServer{
							Name:      name,
							Command:   srv.Command,
							Args:      srv.Args,
							Env:       srv.Env,
							Transport: "stdio",
							Source:    "vscode",
						})
					}
				}
			}
		}
	}

	// Cline
	if clientFilter == "" || clientFilter == "cline" {
		adapter := cline.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCPServers {
					if !seen[name] {
						seen[name] = true
						discovered = append(discovered, DiscoveredServer{
							Name:      name,
							Command:   srv.Command,
							Args:      srv.Args,
							Env:       srv.Env,
							Transport: "stdio",
							Source:    "cline",
						})
					}
				}
			}
		}
	}

	// Warp
	if clientFilter == "" || clientFilter == "warp" {
		adapter := warp.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCPServers {
					if !seen[name] {
						seen[name] = true
						discovered = append(discovered, DiscoveredServer{
							Name:      name,
							Command:   srv.Command,
							Args:      srv.Args,
							Env:       srv.Env,
							Transport: "stdio",
							Source:    "warp",
						})
					}
				}
			}
		}
	}

	// OpenCode - special handling for its format
	if clientFilter == "" || clientFilter == "opencode" {
		adapter := opencode.Adapter{}
		if ok, _ := adapter.Detect(); ok {
			if cfg, err := adapter.Load(); err == nil {
				for name, srv := range cfg.MCP {
					// Check if enabled (nil means enabled by default, or explicit true)
					if srv.Enabled == nil || *srv.Enabled {
						if !seen[name] {
							seen[name] = true
							// OpenCode stores command as array [binary, arg1, arg2, ...]
							var cmd string
							var args []string
							if len(srv.Command) > 0 {
								cmd = srv.Command[0]
								if len(srv.Command) > 1 {
									args = srv.Command[1:]
								}
							}
							transport := "stdio"
							if srv.Type == "remote" {
								transport = "http"
								cmd = srv.URL // For remote, URL is the "command"
								args = nil
							}
							discovered = append(discovered, DiscoveredServer{
								Name:      name,
								Command:   cmd,
								Args:      args,
								Env:       srv.Environment,
								Transport: transport,
								Source:    "opencode",
							})
						}
					}
				}
			}
		}
	}

	return discovered
}
