package main

// proxy_cmd.go — "mseep proxy" subcommand used in wrapper mode.
//
// Each AI client configured in wrapper mode runs:
//   mseep proxy --client <client-name>
// as its single MCP server entry.  mseep then multiplexes all enabled
// canonical servers through this process.
//
// Uses Cobra's init() pattern so this file is self-contained.

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/ResistanceIsUseless/mseep/internal/proxy"
)

var proxyClientFlag string

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the MCP multiplexer proxy (used by wrapper mode)",
	Long: `Start the mseep MCP proxy for a specific client.

In wrapper mode, each AI client is configured to run:

  mseep proxy --client <client-name>

The proxy reads canonical.json, spawns subprocess for each enabled
stdio MCP server, and multiplexes all their tools/resources/prompts
into a single JSON-RPC endpoint over stdin/stdout.

Each client connection is fully isolated — separate proxy processes with
their own child process trees. No session state is shared between clients.

The proxy hot-reloads when canonical.json changes (configurable poll interval).`,
	RunE: runProxy,
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.Flags().StringVar(&proxyClientFlag, "client", "",
		"Client name this proxy is serving (e.g. claude-code, cursor, opencode)")
	// --client is optional but recommended for logging/debugging.
}

func runProxy(_ *cobra.Command, _ []string) error {
	clientName := proxyClientFlag
	if clientName == "" {
		clientName = "unknown"
	}

	// Log to stderr only — stdout is the MCP JSON-RPC channel.
	fmt.Fprintf(os.Stderr, "[mseep proxy] starting for client=%s pid=%d\n", clientName, os.Getpid())

	// Create the proxy (loads canonical, starts server subprocesses).
	p, err := proxy.New(clientName, "")
	if err != nil {
		return fmt.Errorf("failed to initialise proxy: %w", err)
	}
	defer p.Shutdown()

	// Set up context so we can shut down cleanly on SIGTERM/SIGINT.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "[mseep proxy] received signal, shutting down\n")
		cancel()
	}()

	// Start config polling in the background for hot-reload.
	go p.PollConfig(ctx)

	// Run the main JSON-RPC loop.  Blocks until client closes stdin or ctx
	// is cancelled.
	if err := p.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "[mseep proxy] exiting with error: %v\n", err)
		return err
	}

	fmt.Fprintf(os.Stderr, "[mseep proxy] done\n")
	return nil
}
