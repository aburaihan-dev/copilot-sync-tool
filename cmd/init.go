package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up a new dotfiles repo and capture your current Copilot config",
	Long: `Guides you through creating a new dotfiles repository for syncing GitHub Copilot
configuration across devices.

Steps performed:
  1. Choose a local directory for the dotfiles repo
  2. Scaffold the expected folder structure
  3. Optionally initialise a git repository and add a remote
  4. Capture your current Copilot config (agents, MCP config, settings) into it
  5. Optionally commit and push to the remote`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// emptyMCPConfig is a valid, empty MCP server config.
const emptyMCPConfig = `{
  "mcpServers": {}
}
`

func runInit(cmd *cobra.Command, args []string) error {
	ui.Header("Initialise Copilot Dotfiles Repo")
	fmt.Println()

	// ── Step 1: choose directory ────────────────────────────────────────────
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	defaultDir := filepath.Join(home, "copilot-dotfiles")

	var chosenDir string
	if err := survey.AskOne(&survey.Input{
		Message: "Where should the dotfiles repo be created?",
		Default: defaultDir,
		Help:    "This directory will be created if it does not exist.",
	}, &chosenDir, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	chosenDir = filepath.Clean(chosenDir)

	// ── Step 2: scaffold structure ──────────────────────────────────────────
	ui.SectionHeader("Scaffolding directory structure")
	fmt.Println()

	dirs := []string{
		chosenDir,
		filepath.Join(chosenDir, "copilot"),
		filepath.Join(chosenDir, "copilot", "agents"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", d, err)
		}
		ui.Success(fmt.Sprintf("Created %s", shortenPath(d, home)))
	}

	// Create empty MCP config files for all platforms if they don't exist.
	for _, plat := range []string{"windows", "macos", "linux"} {
		p := platform.DotfilesMCPConfig(chosenDir, plat)
		if err := writeIfAbsent(p, emptyMCPConfig); err != nil {
			return err
		}
		ui.Success(fmt.Sprintf("Created %s", shortenPath(p, home)))
	}

	// Create empty settings.json if absent.
	settingsPath := platform.DotfilesSettingsFile(chosenDir)
	if err := writeIfAbsent(settingsPath, "{}\n"); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Created %s", shortenPath(settingsPath, home)))

	// Create empty copilot-instructions.md if absent.
	instrPath := platform.DotfilesInstructionsFile(chosenDir)
	if err := writeIfAbsent(instrPath, "# Copilot Instructions\n\n<!-- Add your global instructions here -->\n"); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Created %s", shortenPath(instrPath, home)))
	fmt.Println()

	// ── Step 3: git init ────────────────────────────────────────────────────
	ui.SectionHeader("Git setup")
	fmt.Println()

	var doGitInit bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Initialise a git repository in this directory?",
		Default: true,
	}, &doGitInit); err != nil {
		return err
	}

	var remoteURL string
	if doGitInit {
		if err := gitRun(chosenDir, "init"); err != nil {
			ui.Warn(fmt.Sprintf("git init failed: %v", err))
		} else {
			ui.Success("git repository initialised")
		}

		if err := survey.AskOne(&survey.Input{
			Message: "Remote GitHub repo URL (leave blank to skip):",
			Help:    "e.g. https://github.com/yourname/copilot-dotfiles.git",
		}, &remoteURL); err != nil {
			return err
		}
		remoteURL = strings.TrimSpace(remoteURL)
		if remoteURL != "" {
			if err := gitRun(chosenDir, "remote", "add", "origin", remoteURL); err != nil {
				ui.Warn(fmt.Sprintf("Could not add remote: %v", err))
			} else {
				ui.Success(fmt.Sprintf("Remote origin set to %s", remoteURL))
			}
		}
	}
	fmt.Println()

	// ── Step 4: capture existing config ────────────────────────────────────
	ui.SectionHeader("Capture existing Copilot config")
	fmt.Println()

	copilotDir, err := platform.CopilotDir()
	if err != nil {
		return err
	}
	ui.Info(fmt.Sprintf("Detected Copilot config dir: %s", copilotDir))
	fmt.Println()

	var doCapture bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Capture your current Copilot config into the dotfiles repo now?",
		Default: true,
		Help:    "This copies agents, MCP config, and settings into the new repo.",
	}, &doCapture); err != nil {
		return err
	}

	if doCapture {
		// Temporarily override the dotfiles dir for the capture sub-command.
		dotfilesDir = chosenDir
		capturePush = false // don't auto-push yet — we ask below
		if err := runCapture(cmd, args); err != nil {
			ui.Warn(fmt.Sprintf("Capture encountered issues: %v", err))
		}
		fmt.Println()
	}

	// ── Step 5: initial commit & push ───────────────────────────────────────
	if doGitInit {
		ui.SectionHeader("Initial commit")
		fmt.Println()

		var doCommit bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Create an initial git commit?",
			Default: true,
		}, &doCommit); err != nil {
			return err
		}

		if doCommit {
			_ = gitRun(chosenDir, "add", "-A")
			if err := gitRun(chosenDir, "commit", "-m", "chore: initial copilot dotfiles"); err != nil {
				ui.Warn(fmt.Sprintf("Commit failed: %v", err))
			} else {
				ui.Success("Initial commit created")
			}

			if remoteURL != "" {
				var doPush bool
				if err := survey.AskOne(&survey.Confirm{
					Message: "Push to remote now?",
					Default: true,
				}, &doPush); err != nil {
					return err
				}
				if doPush {
					ui.Action("Pushing to remote...")
					if err := gitRunWithOutput(chosenDir, "push", "-u", "origin", "HEAD"); err != nil {
						ui.Warn(fmt.Sprintf("Push failed: %v", err))
					} else {
						ui.Success("Pushed successfully")
					}
				}
			}
		}
		fmt.Println()
	}

	// ── Done ────────────────────────────────────────────────────────────────
	ui.Header("Setup complete")
	fmt.Println()
	ui.Info(fmt.Sprintf("Dotfiles repo: %s", chosenDir))
	ui.Info("Next steps:")
	fmt.Printf("  %s Run 'copilot-sync-tool install' to symlink config to this device\n", ui.ActionMark)
	fmt.Printf("  %s Run 'copilot-sync-tool status'  to verify the sync state\n", ui.ActionMark)
	fmt.Printf("  %s On a new machine: clone the repo and run 'copilot-sync-tool install'\n", ui.ActionMark)
	fmt.Println()

	return nil
}

// writeIfAbsent writes content to path only if the file does not already exist.
func writeIfAbsent(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// shortenPath replaces the home directory prefix with ~.
func shortenPath(p, home string) string {
	rel, err := filepath.Rel(home, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return "~" + string(filepath.Separator) + rel
}

// gitRun runs a git sub-command in dir, discarding output.
func gitRun(dir string, args ...string) error {
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitRunWithOutput runs a git sub-command and prints output to stdout.
func gitRunWithOutput(dir string, args ...string) error {
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
