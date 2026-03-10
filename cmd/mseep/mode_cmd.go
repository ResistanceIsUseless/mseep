package main

// mode_cmd.go — "mseep mode" subcommands for reading and writing the global
// delivery mode (basic | wrapper).
//
// Usage:
//   mseep mode get              – print current mode
//   mseep mode set basic        – switch to basic (direct config entries)
//   mseep mode set wrapper      – switch to wrapper (single mseep proxy entry)
//   mseep mode set wrapper --poll 10            – set poll interval
//   mseep mode set wrapper --namespace=false    – disable tool namespacing

import (
	"fmt"
	"os"

	"github.com/ResistanceIsUseless/mseep/internal/config"
	"github.com/ResistanceIsUseless/mseep/internal/style"
	"github.com/spf13/cobra"
)

func cmdMode() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode",
		Short: "Get or set the MCP delivery mode (basic|wrapper)",
		Long: `Controls how mseep delivers enabled MCP servers to each client.

  basic   – write individual server entries directly into each client's config
            file. Simple, no runtime dependency on mseep. (default)

  wrapper – write a single "mseep proxy" entry into each client's config.
            mseep itself multiplexes all enabled servers at runtime, allowing
            hot-reload and unified tool namespacing without restarting clients.`,
	}

	cmd.AddCommand(cmdModeGet(), cmdModeSet())
	return cmd
}

func cmdModeGet() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Print the current delivery mode",
		RunE:  runModeGet,
	}
}

func cmdModeSet() *cobra.Command {
	var (
		pollInterval int
		namespace    bool
		separator    string
	)

	cmd := &cobra.Command{
		Use:   "set <basic|wrapper>",
		Short: "Set the delivery mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModeSet(args[0], pollInterval, namespace, separator)
		},
	}

	cmd.Flags().IntVar(&pollInterval, "poll", 5,
		"Wrapper poll interval in seconds (0 = load once at startup)")
	cmd.Flags().BoolVar(&namespace, "namespace", true,
		"Prefix tool names with server name to avoid collisions (wrapper mode)")
	cmd.Flags().StringVar(&separator, "separator", "__",
		"Separator used for tool name namespacing (wrapper mode)")

	return cmd
}

func runModeGet(_ *cobra.Command, _ []string) error {
	canon, err := loadCanon()
	if err != nil {
		return err
	}
	mode := canon.EffectiveMode()
	fmt.Print(style.Header("MCP Delivery Mode") + "\n\n")
	fmt.Print(style.Success("Mode: ") + mode + "\n")

	if mode == "wrapper" {
		ws := canon.Settings.Wrapper
		fmt.Print(style.Muted(fmt.Sprintf(
			"  poll interval : %d s\n  namespace tools: %v\n  separator      : %q\n",
			canon.EffectivePollInterval(),
			ws.NamespaceTools,
			canon.EffectiveNamespaceSeparator(),
		)))
	}
	return nil
}

func runModeSet(requested string, pollInterval int, namespace bool, separator string) error {
	if requested != "basic" && requested != "wrapper" {
		return fmt.Errorf("mode must be 'basic' or 'wrapper', got %q", requested)
	}

	canon, err := loadCanon()
	if err != nil {
		return err
	}

	// Apply mode and wrapper settings.
	canon.Settings.Mode = requested

	if requested == "wrapper" {
		canon.Settings.Wrapper.PollIntervalSeconds = pollInterval
		canon.Settings.Wrapper.NamespaceTools = namespace
		canon.Settings.Wrapper.NamespaceSeparator = separator
	}

	if err := config.Save("", canon); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Print(style.Success(fmt.Sprintf("Mode set to %q", requested)) + "\n")

	if requested == "wrapper" {
		fmt.Print(style.Warning(
			"\nRun 'mseep apply' to update client configs to use the mseep proxy entry.\n",
		))
	} else {
		fmt.Print(style.Warning(
			"\nRun 'mseep apply' to write individual server entries back to client configs.\n",
		))
	}
	return nil
}

// loadCanon is a thin helper shared across cmd files.
// It loads the canonical config from the default path.
func loadCanon() (*config.Canonical, error) {
	canon, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load canonical config: %v\n", err)
		return nil, err
	}
	return canon, nil
}
