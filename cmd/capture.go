package cmd

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/agents"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/git"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/mcp"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/secrets"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/symlink"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var (
	captureMCP          bool
	captureAgents       bool
	captureSettings     bool
	captureInstructions bool
	capturePush         bool
	captureMessage      string
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture local Copilot config into dotfiles repo",
	Long: `Copies local GitHub Copilot configuration into the dotfiles repo so it can
be versioned and synced to other machines.

Files captured (when no filter flag is given, all are included):
  agents/                 — custom agent definition files (.agent.md)
  mcp-config.json         — MCP server configuration (per-platform)
  settings.json           — Copilot CLI settings
  copilot-instructions.md — global Copilot instructions

Files already managed as symlinks are automatically skipped.
After a successful capture, the dotfiles repo is committed and pushed
unless --no-push is specified.`,
	Example: `  # Capture everything and push to git
  copilot-sync-tool capture

  # Capture only agents, skip git push
  copilot-sync-tool capture --agents --no-push

  # Capture MCP config with a custom commit message
  copilot-sync-tool capture --mcp --message "feat: add new MCP servers"

  # Capture from a custom dotfiles location
  copilot-sync-tool capture --dotfiles ~/my-dotfiles`,
	RunE: runCapture,
}

func init() {
	captureCmd.Flags().BoolVar(&captureMCP, "mcp", false, "Capture only mcp-config.json")
	captureCmd.Flags().BoolVar(&captureAgents, "agents", false, "Capture only agents/")
	captureCmd.Flags().BoolVar(&captureSettings, "settings", false, "Capture only settings.json")
	captureCmd.Flags().BoolVar(&captureInstructions, "instructions", false, "Capture only copilot-instructions.md")
	captureCmd.Flags().BoolVar(&capturePush, "push", true, "Git commit and push after capture (disable with --no-push)")
	captureCmd.Flags().StringVar(&captureMessage, "message", "", "Custom git commit message (auto-generated if not set)")
	rootCmd.AddCommand(captureCmd)
}

func runCapture(cmd *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(dotfiles); os.IsNotExist(err) {
		return fmt.Errorf("dotfiles directory does not exist: %s", dotfiles)
	}

	plat := platform.Current()

	// If no specific flag set, capture all
	captureAll := !captureMCP && !captureAgents && !captureSettings && !captureInstructions

	ui.Header("Capturing Copilot Config")
	fmt.Println()

	var captured []string
	var skipped []string

	// Capture agents
	if captureAll || captureAgents {
		ui.SectionHeader("Agents")
		agentsLocal, err := platform.AgentsDir()
		if err != nil {
			return fmt.Errorf("resolving agents dir: %w", err)
		}
		agentsDotfiles := platform.DotfilesAgentsDir(dotfiles)

		isLink, _ := symlink.Check(agentsLocal)
		if isLink {
			ui.Skip("agents/ - already symlinked to dotfiles")
			skipped = append(skipped, "agents/")
		} else {
			if err := os.MkdirAll(agentsDotfiles, 0755); err != nil {
				ui.Error(fmt.Sprintf("Failed to create agents dotfiles dir: %v", err))
			} else {
				newAgents, err := agents.CaptureNew(agentsLocal, agentsDotfiles)
				if err != nil {
					ui.Error(fmt.Sprintf("Failed to capture agents: %v", err))
				} else if len(newAgents) == 0 {
					ui.Info("No new agents to capture")
					skipped = append(skipped, "agents/ (no new files)")
				} else {
					for _, a := range newAgents {
						ui.Success(fmt.Sprintf("Captured agent: %s", a))
						captured = append(captured, "agents/"+a)
					}
				}
			}
		}
	}

	// Capture MCP config
	if captureAll || captureMCP {
		fmt.Println()
		ui.SectionHeader("MCP Config")
		mcpLocal, err := platform.MCPConfigFile()
		if err != nil {
			return fmt.Errorf("resolving MCP config file: %w", err)
		}
		mcpDotfiles := platform.DotfilesMCPConfig(dotfiles, plat)

		isLink, _ := symlink.Check(mcpLocal)
		if isLink {
			ui.Skip("mcp-config.json - already symlinked to dotfiles")
			skipped = append(skipped, "mcp-config.json")
		} else {
			if _, err := os.Stat(mcpLocal); os.IsNotExist(err) {
				ui.Warn("mcp-config.json does not exist locally - skipping")
				skipped = append(skipped, "mcp-config.json (not found)")
			} else {
				added, err := mcp.CaptureInto(mcpLocal, mcpDotfiles)
				if err != nil {
					ui.Error(fmt.Sprintf("Failed to capture MCP config: %v", err))
				} else if len(added) == 0 {
					ui.Info("No new MCP servers to capture")
					skipped = append(skipped, "mcp-config.json (no new servers)")
				} else {
					for _, s := range added {
						ui.Success(fmt.Sprintf("Added MCP server: %s", s))
					}
					captured = append(captured, "mcp-config."+plat+".json")
				}
			}
		}
	}

	// Capture settings
	if captureAll || captureSettings {
		fmt.Println()
		ui.SectionHeader("Settings")
		settingsLocal, err := platform.SettingsFile()
		if err != nil {
			return fmt.Errorf("resolving settings file: %w", err)
		}
		settingsDotfiles := platform.DotfilesSettingsFile(dotfiles)

		isLink, _ := symlink.Check(settingsLocal)
		if isLink {
			ui.Skip("settings.json - already symlinked to dotfiles")
			skipped = append(skipped, "settings.json")
		} else {
			if _, err := os.Stat(settingsLocal); os.IsNotExist(err) {
				ui.Warn("settings.json does not exist locally - skipping")
				skipped = append(skipped, "settings.json (not found)")
			} else {
				changed, err := captureFileIfChanged(settingsLocal, settingsDotfiles)
				if err != nil {
					ui.Error(fmt.Sprintf("Failed to capture settings: %v", err))
				} else if changed {
					ui.Success("Captured settings.json")
					captured = append(captured, "settings.json")
				} else {
					ui.Info("settings.json unchanged")
					skipped = append(skipped, "settings.json (unchanged)")
				}
			}
		}
	}

	// Capture instructions
	if captureAll || captureInstructions {
		fmt.Println()
		ui.SectionHeader("Instructions")
		instrLocal, err := platform.InstructionsFile()
		if err != nil {
			return fmt.Errorf("resolving instructions file: %w", err)
		}
		instrDotfiles := platform.DotfilesInstructionsFile(dotfiles)

		isLink, _ := symlink.Check(instrLocal)
		if isLink {
			ui.Skip("copilot-instructions.md - already symlinked to dotfiles")
			skipped = append(skipped, "copilot-instructions.md")
		} else {
			if _, err := os.Stat(instrLocal); os.IsNotExist(err) {
				ui.Warn("copilot-instructions.md does not exist locally - skipping")
				skipped = append(skipped, "copilot-instructions.md (not found)")
			} else {
				changed, err := captureFileIfChanged(instrLocal, instrDotfiles)
				if err != nil {
					ui.Error(fmt.Sprintf("Failed to capture instructions: %v", err))
				} else if changed {
					ui.Success("Captured copilot-instructions.md")
					captured = append(captured, "copilot-instructions.md")
				} else {
					ui.Info("copilot-instructions.md unchanged")
					skipped = append(skipped, "copilot-instructions.md (unchanged)")
				}
			}
		}
	}

	// Summary
	fmt.Println()
	ui.SectionHeader("Summary")
	ui.Info(fmt.Sprintf("Captured: %d items", len(captured)))
	ui.Info(fmt.Sprintf("Skipped:  %d items", len(skipped)))

	if len(captured) == 0 {
		ui.Info("Nothing new to commit")
		return nil
	}

	// Secrets scan on captured JSON files
	if err := runSecretsCheck(dotfiles, plat, captured); err != nil {
		return err
	}

	// Git commit + push
	if capturePush {
		fmt.Println()
		ui.SectionHeader("Git")
		msg := captureMessage
		if msg == "" {
			msg = fmt.Sprintf("chore: capture %s config [%s]", plat, platform.Hostname())
		}
		if err := git.AddAll(dotfiles); err != nil {
			return fmt.Errorf("git add failed: %w", err)
		}
		if err := git.Commit(dotfiles, msg); err != nil {
			ui.Warn(fmt.Sprintf("Git commit: %v (nothing to commit?)", err))
		} else {
			ui.Success(fmt.Sprintf("Committed: %s", msg))
			if err := git.Push(dotfiles); err != nil {
				ui.Warn(fmt.Sprintf("Git push failed: %v", err))
			} else {
				ui.Success("Pushed to remote")
			}
		}
	}

	return nil
}

// runSecretsCheck scans all captured JSON files for potential secrets and warns
// (or optionally aborts) before commit.
func runSecretsCheck(dotfiles, plat string, captured []string) error {
	// Collect JSON files that were just captured
	var filesToScan []string
	for _, item := range captured {
		if item == "mcp-config."+plat+".json" {
			filesToScan = append(filesToScan, platform.DotfilesMCPConfig(dotfiles, plat))
		}
		if item == "settings.json" {
			filesToScan = append(filesToScan, platform.DotfilesSettingsFile(dotfiles))
		}
	}
	if len(filesToScan) == 0 {
		return nil
	}

	var allFindings []secrets.Finding
	for _, f := range filesToScan {
		found, err := secrets.ScanJSONFile(f)
		if err != nil {
			continue
		}
		allFindings = append(allFindings, found...)
	}

	if len(allFindings) == 0 {
		return nil
	}

	fmt.Println()
	ui.SectionHeader("Secrets Check")
	ui.Warn(fmt.Sprintf("Potential secret(s) detected in files to be committed (%d finding(s)):", len(allFindings)))
	for _, f := range allFindings {
		ui.Item(fmt.Sprintf("%-40s key=%s  value=%s", f.File, f.Key, f.Snippet))
	}
	fmt.Println()
	ui.Info("Tip: replace secret values with environment variable placeholders, e.g. ${MY_TOKEN}")

	var proceed bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Commit anyway?",
		Default: false,
	}, &proceed); err != nil {
		return err
	}
	if !proceed {
		return fmt.Errorf("commit aborted by user (potential secrets detected)")
	}
	return nil
}

// captureFileIfChanged copies src to dst if they differ. Returns true if a copy happened.
func captureFileIfChanged(src, dst string) (bool, error) {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", src, err)
	}

	if dstData, err := os.ReadFile(dst); err == nil {
		if string(srcData) == string(dstData) {
			return false, nil
		}
	}

	if err := os.WriteFile(dst, srcData, 0644); err != nil {
		return false, fmt.Errorf("writing %s: %w", dst, err)
	}
	return true, nil
}
