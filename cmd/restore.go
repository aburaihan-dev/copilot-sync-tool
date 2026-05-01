package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore files from .bak backups created by install",
	Long: `Finds all .bak backup files created by 'copilot-sync-tool install' under
the Copilot config directory and lets you pick which originals to restore.

Backups are created automatically before any file is overwritten during
install. Use this command if you want to roll back to a previous state.`,
	Example: `  # Interactive restore picker
  copilot-sync-tool restore

  # Restore all backups without prompting
  copilot-sync-tool restore --all`,
	RunE: runRestore,
}

var restoreAll bool

func init() {
	restoreCmd.Flags().BoolVar(&restoreAll, "all", false, "Restore all backups without prompting")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(_ *cobra.Command, _ []string) error {
	ui.Header("Restore Backups")
	fmt.Println()

	copilotDir, err := platform.CopilotDir()
	if err != nil {
		return err
	}

	baks, err := findBackups(copilotDir)
	if err != nil {
		return fmt.Errorf("scanning for backups: %w", err)
	}

	if len(baks) == 0 {
		ui.Info("No .bak files found. Nothing to restore.")
		return nil
	}

	ui.Info(fmt.Sprintf("Found %d backup(s) under %s", len(baks), copilotDir))
	fmt.Println()

	var selected []string

	if restoreAll {
		for _, b := range baks {
			selected = append(selected, b)
		}
	} else {
		// Build display labels
		labels := make([]string, len(baks))
		for i, b := range baks {
			rel, _ := filepath.Rel(copilotDir, b)
			labels[i] = rel
		}

		var chosen []string
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "Select backups to restore:",
			Options: labels,
		}, &chosen); err != nil {
			return err
		}

		// Map chosen labels back to full paths
		labelToPath := map[string]string{}
		for i, l := range labels {
			labelToPath[l] = baks[i]
		}
		for _, c := range chosen {
			selected = append(selected, labelToPath[c])
		}
	}

	if len(selected) == 0 {
		ui.Info("Nothing selected.")
		return nil
	}

	var errs []string
	for _, bak := range selected {
		original := strings.TrimSuffix(bak, ".bak")
		if err := os.Rename(bak, original); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", bak, err))
			ui.Error(fmt.Sprintf("Failed to restore %s: %v", filepath.Base(original), err))
		} else {
			ui.Success(fmt.Sprintf("Restored %s", original))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d restore(s) failed", len(errs))
	}
	return nil
}

// findBackups walks dir recursively and returns all *.bak files.
func findBackups(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() && strings.HasSuffix(path, ".bak") {
			result = append(result, path)
		}
		return nil
	})
	return result, err
}
