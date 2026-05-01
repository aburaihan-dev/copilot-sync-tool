package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/mcp"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/symlink"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var mergeAI bool

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Intelligently merge MCP configs between local and dotfiles",
	Long: `Compares the local mcp-config.json with the dotfiles version and interactively
resolves differences.

For each server that exists only locally you are asked whether to add it to
the dotfiles. For servers that exist in both but differ, you choose which
version to keep. Servers that exist only in the dotfiles are always preserved.

The merged result is written back to the dotfiles repo. Use --ai to generate
a merge prompt and optionally invoke the 'copilot' CLI for AI-assisted review.

This command has no effect when the local mcp-config.json is already a
symlink to the dotfiles (nothing to merge in that case).`,
	Example: `  # Interactive merge (default)
  copilot-sync-tool merge

  # Merge with AI-assisted prompt generation
  copilot-sync-tool merge --ai

  # Merge against a custom dotfiles location
  copilot-sync-tool merge --dotfiles ~/my-dotfiles`,
	RunE: runMerge,
}

func init() {
	mergeCmd.Flags().BoolVar(&mergeAI, "ai", false, "Generate an AI merge prompt and optionally invoke the copilot CLI")
	rootCmd.AddCommand(mergeCmd)
}

func runMerge(cmd *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	plat := platform.Current()
	mcpLocal, err := platform.MCPConfigFile()
	if err != nil {
		return fmt.Errorf("resolving MCP config file: %w", err)
	}
	mcpDotfiles := platform.DotfilesMCPConfig(dotfiles, plat)

	ui.Header("Merging MCP Configs")
	fmt.Println()

	// Check if local is a symlink (already managed)
	isLink, _ := symlink.Check(mcpLocal)
	if isLink {
		ui.Info("Local mcp-config.json is already a symlink to dotfiles - nothing to merge")
		return nil
	}

	// Load configs
	localCfg, err := mcp.Load(mcpLocal)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Warn("No local mcp-config.json found")
			return nil
		}
		return fmt.Errorf("loading local MCP config: %w", err)
	}

	dotfilesCfg, err := mcp.Load(mcpDotfiles)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Warn(fmt.Sprintf("No dotfiles MCP config for %s - will create it", plat))
			dotfilesCfg = mcp.NewConfig()
		} else {
			return fmt.Errorf("loading dotfiles MCP config: %w", err)
		}
	}

	// Compute diff
	diff := mcp.Diff(localCfg, dotfilesCfg)

	ui.SectionHeader("Diff Summary")
	fmt.Printf("  Servers only in local:    %d\n", len(diff.OnlyLocal))
	fmt.Printf("  Servers only in dotfiles: %d\n", len(diff.OnlyDotfiles))
	fmt.Printf("  Servers in both (same):   %d\n", len(diff.Same))
	fmt.Printf("  Servers in both (differ): %d\n", len(diff.Different))
	fmt.Println()

	if len(diff.OnlyLocal) == 0 && len(diff.OnlyDotfiles) == 0 && len(diff.Different) == 0 {
		ui.Success("Configs are already in sync - nothing to merge")
		return nil
	}

	merged := mcp.Clone(dotfilesCfg)

	// Handle servers only in local
	if len(diff.OnlyLocal) > 0 {
		ui.SectionHeader("Servers only in local")
		for _, name := range diff.OnlyLocal {
			var addIt bool
			confirmPrompt := &survey.Confirm{
				Message: fmt.Sprintf("Add '%s' to dotfiles?", name),
				Default: true,
			}
			if err := survey.AskOne(confirmPrompt, &addIt); err != nil {
				return err
			}
			if addIt {
				merged.Servers[name] = localCfg.Servers[name]
				ui.Success(fmt.Sprintf("Added: %s", name))
			} else {
				ui.Skip(fmt.Sprintf("Skipped: %s", name))
			}
		}
		fmt.Println()
	}

	// Handle servers in both but different
	if len(diff.Different) > 0 {
		ui.SectionHeader("Servers with conflicting configs")
		for _, name := range diff.Different {
			ui.Info(fmt.Sprintf("Conflict: %s", name))
			localJSON, _ := mcp.PrettyServer(localCfg, name)
			dotfilesJSON, _ := mcp.PrettyServer(dotfilesCfg, name)
			fmt.Printf("  Local:\n%s\n", indentString(localJSON, "    "))
			fmt.Printf("  Dotfiles:\n%s\n", indentString(dotfilesJSON, "    "))

			var choice string
			selectPrompt := &survey.Select{
				Message: fmt.Sprintf("Which version of '%s' to keep?", name),
				Options: []string{"dotfiles", "local", "skip"},
				Default: "dotfiles",
			}
			if err := survey.AskOne(selectPrompt, &choice); err != nil {
				return err
			}
			switch choice {
			case "local":
				merged.Servers[name] = localCfg.Servers[name]
				ui.Success(fmt.Sprintf("Using local version of: %s", name))
			case "dotfiles":
				// already in merged (from Clone)
				ui.Info(fmt.Sprintf("Keeping dotfiles version of: %s", name))
			case "skip":
				ui.Skip(fmt.Sprintf("Skipped: %s", name))
			}
		}
		fmt.Println()
	}

	// Show what's only in dotfiles
	if len(diff.OnlyDotfiles) > 0 {
		ui.SectionHeader("Servers only in dotfiles (preserved)")
		for _, name := range diff.OnlyDotfiles {
			ui.Info(fmt.Sprintf("  %s", name))
		}
		fmt.Println()
	}

	// Write merged result
	if err := mcp.Save(merged, mcpDotfiles); err != nil {
		return fmt.Errorf("saving merged config: %w", err)
	}
	ui.Success(fmt.Sprintf("Merged config written to %s", mcpDotfiles))

	// AI flag
	if mergeAI {
		fmt.Println()
		ui.SectionHeader("AI Merge Assist")
		aiPrompt := buildAIPrompt(diff, plat)
		fmt.Println("AI Merge Prompt:")
		fmt.Println("────────────────")
		fmt.Println(aiPrompt)
		fmt.Println("────────────────")

		if copilotPath, err := exec.LookPath("copilot"); err == nil {
			var launch bool
			launchPrompt := &survey.Confirm{
				Message: fmt.Sprintf("Launch 'copilot' CLI (%s) with this prompt?", copilotPath),
				Default: false,
			}
			if err := survey.AskOne(launchPrompt, &launch); err == nil && launch {
				c := exec.Command("copilot", aiPrompt)
				c.Stdin = os.Stdin
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				if err := c.Run(); err != nil {
					ui.Warn(fmt.Sprintf("copilot CLI exited: %v", err))
				}
			}
		} else {
			ui.Info("'copilot' binary not found in PATH - copy the prompt above to use with GitHub Copilot")
		}
	}

	return nil
}

func buildAIPrompt(diff mcp.DiffResult, plat string) string {
	return fmt.Sprintf(`I am merging GitHub Copilot MCP server configurations between a local machine (%s) and a dotfiles repo.

Servers only in local (not yet in dotfiles): %v
Servers only in dotfiles (not on this machine): %v
Servers in both with different configs: %v

Please help me decide which servers to include in the merged dotfiles config and how to resolve any conflicts.
The goal is a dotfiles config that works across macOS, Linux, and Windows.`,
		plat, diff.OnlyLocal, diff.OnlyDotfiles, diff.Different)
}

func indentString(s, prefix string) string {
	result := ""
	for _, line := range splitLines(s) {
		result += prefix + line + "\n"
	}
	return result
}
