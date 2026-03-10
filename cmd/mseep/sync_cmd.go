package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ResistanceIsUseless/mseep/internal/style"
)

func cmdSync() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Configure iCloud sync for canonical config",
		Long: `Set up iCloud Drive sync for your mseep canonical configuration.

This creates a symlink from the local config location to iCloud Drive,
allowing your MCP server configuration to sync across all your Macs.

The canonical.json file will be stored at:
  ~/Library/Mobile Documents/com~apple~CloudDocs/mseep/canonical.json

And symlinked from:
  ~/Library/Application Support/mseep/canonical.json

Examples:
  # Enable iCloud sync
  mseep sync enable

  # Disable iCloud sync (move config back to local)
  mseep sync disable

  # Check current sync status
  mseep sync status`,
	}

	cmd.AddCommand(cmdSyncEnable())
	cmd.AddCommand(cmdSyncDisable())
	cmd.AddCommand(cmdSyncStatus())

	return cmd
}

func cmdSyncEnable() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable iCloud sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("iCloud sync is only available on macOS")
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			// Paths
			icloudDir := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "mseep")
			icloudConfig := filepath.Join(icloudDir, "canonical.json")
			localDir := filepath.Join(home, "Library", "Application Support", "mseep")
			localConfig := filepath.Join(localDir, "canonical.json")

			// Check if iCloud Drive exists
			icloudBase := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
			if _, err := os.Stat(icloudBase); os.IsNotExist(err) {
				return fmt.Errorf("iCloud Drive not found at %s", icloudBase)
			}

			// Create iCloud mseep directory
			if err := os.MkdirAll(icloudDir, 0o755); err != nil {
				return fmt.Errorf("failed to create iCloud directory: %w", err)
			}

			// Check current state
			localInfo, localErr := os.Lstat(localConfig)
			icloudExists := false
			if _, err := os.Stat(icloudConfig); err == nil {
				icloudExists = true
			}

			// If local config exists and is a symlink, check where it points
			if localErr == nil && localInfo.Mode()&os.ModeSymlink != 0 {
				target, _ := os.Readlink(localConfig)
				if target == icloudConfig {
					fmt.Print(style.Success("iCloud sync is already enabled") + "\n")
					fmt.Print(style.Muted("Config location: ") + style.Code(icloudConfig) + "\n")
					return nil
				}
			}

			// If local config exists and is a regular file, move it to iCloud
			if localErr == nil && localInfo.Mode().IsRegular() {
				if icloudExists {
					// Both exist - ask user what to do
					fmt.Print(style.Warning("Both local and iCloud configs exist") + "\n")
					fmt.Print(style.Muted("Local:  ") + style.Code(localConfig) + "\n")
					fmt.Print(style.Muted("iCloud: ") + style.Code(icloudConfig) + "\n")
					fmt.Print("\nThe local config will be backed up and iCloud config will be used.\n")
					fmt.Print("Continue? [y/N]: ")

					var response string
					fmt.Scanln(&response)
					if response != "y" && response != "Y" {
						return fmt.Errorf("aborted")
					}

					// Backup local
					backupPath := localConfig + ".backup"
					if err := os.Rename(localConfig, backupPath); err != nil {
						return fmt.Errorf("failed to backup local config: %w", err)
					}
					fmt.Print(style.Muted("Backed up local config to: ") + style.Code(backupPath) + "\n")
				} else {
					// Move local to iCloud
					if err := os.Rename(localConfig, icloudConfig); err != nil {
						return fmt.Errorf("failed to move config to iCloud: %w", err)
					}
					fmt.Print(style.Success("Moved config to iCloud") + "\n")
				}
			}

			// Ensure local directory exists
			if err := os.MkdirAll(localDir, 0o755); err != nil {
				return fmt.Errorf("failed to create local directory: %w", err)
			}

			// Remove any existing local config (already backed up or moved)
			_ = os.Remove(localConfig)

			// Create symlink
			if err := os.Symlink(icloudConfig, localConfig); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}

			fmt.Print(style.Success("iCloud sync enabled") + "\n")
			fmt.Print(style.Muted("Config location: ") + style.Code(icloudConfig) + "\n")
			fmt.Print(style.Muted("Symlink: ") + style.Code(localConfig) + " -> " + style.Code(icloudConfig) + "\n")

			return nil
		},
	}
}

func cmdSyncDisable() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable iCloud sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			icloudConfig := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "mseep", "canonical.json")
			localDir := filepath.Join(home, "Library", "Application Support", "mseep")
			localConfig := filepath.Join(localDir, "canonical.json")

			// Check if local config is a symlink
			localInfo, err := os.Lstat(localConfig)
			if err != nil {
				return fmt.Errorf("no config found at %s", localConfig)
			}

			if localInfo.Mode()&os.ModeSymlink == 0 {
				fmt.Print(style.Warning("iCloud sync is not enabled (config is not a symlink)") + "\n")
				return nil
			}

			// Read the iCloud config content
			content, err := os.ReadFile(icloudConfig)
			if err != nil {
				return fmt.Errorf("failed to read iCloud config: %w", err)
			}

			// Remove symlink
			if err := os.Remove(localConfig); err != nil {
				return fmt.Errorf("failed to remove symlink: %w", err)
			}

			// Write content to local file
			if err := os.WriteFile(localConfig, content, 0o644); err != nil {
				return fmt.Errorf("failed to write local config: %w", err)
			}

			fmt.Print(style.Success("iCloud sync disabled") + "\n")
			fmt.Print(style.Muted("Config copied to: ") + style.Code(localConfig) + "\n")
			fmt.Print(style.Muted("iCloud copy preserved at: ") + style.Code(icloudConfig) + "\n")

			return nil
		},
	}
}

func cmdSyncStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sync status",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			icloudConfig := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "mseep", "canonical.json")
			localConfig := filepath.Join(home, "Library", "Application Support", "mseep", "canonical.json")

			fmt.Print(style.Header("Sync Status") + "\n\n")

			// Check local config
			localInfo, localErr := os.Lstat(localConfig)
			if localErr != nil {
				fmt.Print(style.Warning("No local config found") + "\n")
				fmt.Print(style.Muted("Expected at: ") + style.Code(localConfig) + "\n")
				return nil
			}

			if localInfo.Mode()&os.ModeSymlink != 0 {
				target, _ := os.Readlink(localConfig)
				if target == icloudConfig {
					fmt.Print(style.Success("iCloud sync: ENABLED") + "\n")
					fmt.Print(style.Muted("Config location: ") + style.Code(icloudConfig) + "\n")
					fmt.Print(style.Muted("Symlink: ") + style.Code(localConfig) + "\n")

					// Check if iCloud file exists
					if _, err := os.Stat(icloudConfig); os.IsNotExist(err) {
						fmt.Print("\n" + style.Warning("Warning: iCloud config file not found (may be syncing)") + "\n")
					}
				} else {
					fmt.Print(style.Warning("Symlink points to unexpected location") + "\n")
					fmt.Print(style.Muted("Target: ") + style.Code(target) + "\n")
				}
			} else {
				fmt.Print(style.Muted("iCloud sync: DISABLED") + "\n")
				fmt.Print(style.Muted("Config location: ") + style.Code(localConfig) + "\n")

				// Check if iCloud copy exists
				if _, err := os.Stat(icloudConfig); err == nil {
					fmt.Print("\n" + style.Warning("Note: An iCloud copy also exists at:") + "\n")
					fmt.Print(style.Code(icloudConfig) + "\n")
				}
			}

			return nil
		},
	}
}
