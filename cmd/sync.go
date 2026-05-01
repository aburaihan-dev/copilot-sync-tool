package cmd

import (
	"fmt"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/git"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull latest dotfiles and install all components (one-shot update)",
	Long: `Equivalent to running:
  git pull           (in the dotfiles repo)
  install --all      (install every component to this machine)

Use this as a daily driver to keep your Copilot config up to date across
all devices.`,
	Example: `  # Full sync (pull + install everything)
  copilot-sync-tool sync

  # Sync without git pull (re-install from current dotfiles state)
  copilot-sync-tool sync --no-pull`,
	RunE: runSync,
}

var syncNoPull bool

func init() {
	syncCmd.Flags().BoolVar(&syncNoPull, "no-pull", false, "Skip git pull, only run install --all")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	ui.Header("Sync")
	fmt.Println()

	// ── Step 1: git pull ─────────────────────────────────────────────────────
	if !syncNoPull {
		ui.SectionHeader("Pulling dotfiles")
		if err := git.Pull(dotfiles); err != nil {
			return fmt.Errorf("git pull failed: %w", err)
		}
		ui.Success("Pulled latest changes.")
		fmt.Println()
	}

	// ── Step 2: install --all ─────────────────────────────────────────────────
	ui.SectionHeader("Installing all components")

	// Temporarily set the install flags and call runInstall
	installAll = true
	installPull = false // already pulled above
	installMCP = false
	installAgents = false
	installSettings = false
	installInstructions = false
	installInteractive = false
	installCopy = false

	return runInstall(cmd, args)
}
