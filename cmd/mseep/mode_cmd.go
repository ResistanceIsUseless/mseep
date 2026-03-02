package main

// mode_cmd.go — "mseep mode" subcommands for reading and writing the global
// delivery mode (basic | wrapper).
//
// Uses Cobra's init() registration pattern so this file is self-contained and
// does not require changes to main.go.
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

	"github.com/spf13/cobra"
	"mseep/internal/config"
	"mseep/internal/style"
)

// modeCmd is the parent "mode" command group.
var modeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Get or set the MCP delivery mode (basic|wrapper)",
	Long: `Controls how mseep delivers enabled MCP servers to each client.

  basic   – write individual server entries directly into each client's config
            file. Simple, no runtime dependency on mseep. (default)

  wrapper – write a single "mseep proxy" entry into each client's config.
            mseep itself multiplexes all enabled servers at runtime, allowing
            hot-reload and unified tool namespacing without restarting clients.`,
}

var modeGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Print the current delivery mode",
	RunE:  runModeGet,
}

var modeSetCmd = &cobra.Command{
	Use:   "set <basic|wrapper>",
	Short: "Set the delivery mode",
	Args:  cobra.ExactArgs(1),
	RunE:  runModeSet,
}

// Flags for "mode set wrapper"
var (
	modePollInterval  int
	modeNamespace     bool
	modeSeparator     string
)

func init() {
	// Register the top-level "mode" command onto rootCmd (defined in main.go).
	rootCmd.AddCommand(modeCmd)
	modeCmd.AddCommand(modeGetCmd)
	modeCmd.AddCommand(modeSetCmd)

	// Wrapper-specific flags on "mode set" (only meaningful when arg is "wrapper").
	modeSetCmd.Flags().IntVar(&modePollInterval, "poll", 5,
		"Wrapper poll interval in seconds (0 = load once at startup)")
	modeSetCmd.Flags().BoolVar(&modeNamespace, "namespace", true,
		"Prefix tool names with server name to avoid collisions (wrapper mode)")
	modeSetCmd.Flags().StringVar(&modeSeparator, "separator", "__",
		"Separator used for tool name namespacing (wrapper mode)")
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

func runModeSet(_ *cobra.Command, args []string) error {
	requested := args[0]
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
		canon.Settings.Wrapper.PollIntervalSeconds = modePollInterval
		canon.Settings.Wrapper.NamespaceTools = modeNamespace
		canon.Settings.Wrapper.NamespaceSeparator = modeSeparator
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
