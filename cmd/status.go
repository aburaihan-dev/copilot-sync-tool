package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/agents"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/git"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/mcp"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/symlink"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status of Copilot config",
	Long: `Displays a summary table of the sync state between the local Copilot config
directory and the dotfiles repo.

For each managed file/directory the status is one of:
  ✓ symlinked   — file is a symlink pointing to the dotfiles repo
  ⚠ not symlinked — file exists locally but is not managed; run 'install'
  ⚠ not installed — dotfiles version exists but local file is missing
  - missing in dotfiles — not yet captured; run 'capture'

Also shows agent and MCP server counts, git branch, and remote sync state.`,
	Example: `  # Show status using auto-detected dotfiles dir
  copilot-sync-tool status

  # Show status for a specific dotfiles location
  copilot-sync-tool status --dotfiles ~/my-dotfiles`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	plat := platform.Current()
	copilotDir, err := platform.CopilotDir()
	if err != nil {
		return err
	}

	ui.Header("Copilot Sync Status")
	fmt.Println()

	// Platform info
	ui.Info(fmt.Sprintf("Platform:    %s", plat))
	ui.Info(fmt.Sprintf("Copilot dir: %s", copilotDir))
	ui.Info(fmt.Sprintf("Dotfiles:    %s", dotfiles))
	fmt.Println()

	// Check dotfiles dir exists
	if _, err := os.Stat(dotfiles); os.IsNotExist(err) {
		ui.Warn(fmt.Sprintf("Dotfiles directory does not exist: %s", dotfiles))
		return nil
	}

	// File sync status table
	ui.SectionHeader("File Sync Status")
	rows := [][]string{{"File", "Status", "Details"}}

	// settings.json
	settingsLocal, err := platform.SettingsFile()
	if err != nil {
		return fmt.Errorf("resolving settings file: %w", err)
	}
	settingsDotfiles := platform.DotfilesSettingsFile(dotfiles)
	rows = append(rows, symlinkStatusRow("settings.json", settingsLocal, settingsDotfiles))

	// mcp-config.json
	mcpLocal, err := platform.MCPConfigFile()
	if err != nil {
		return fmt.Errorf("resolving MCP config file: %w", err)
	}
	mcpDotfiles := platform.DotfilesMCPConfig(dotfiles, plat)
	rows = append(rows, symlinkStatusRow("mcp-config.json", mcpLocal, mcpDotfiles))

	// agents dir
	agentsLocal, err := platform.AgentsDir()
	if err != nil {
		return fmt.Errorf("resolving agents dir: %w", err)
	}
	agentsDotfiles := platform.DotfilesAgentsDir(dotfiles)
	rows = append(rows, symlinkStatusRow("agents/", agentsLocal, agentsDotfiles))

	// copilot-instructions.md
	instrLocal, err := platform.InstructionsFile()
	if err != nil {
		return fmt.Errorf("resolving instructions file: %w", err)
	}
	instrDotfiles := platform.DotfilesInstructionsFile(dotfiles)
	rows = append(rows, symlinkStatusRow("copilot-instructions.md", instrLocal, instrDotfiles))

	ui.Table(rows)
	fmt.Println()

	// Agent counts
	ui.SectionHeader("Agent Counts")
	localAgentsList, err := agents.List(agentsLocal)
	if err != nil && !os.IsNotExist(err) {
		ui.Warn(fmt.Sprintf("Could not read local agents: %v", err))
	}
	dotfileAgentsList, err := agents.List(agentsDotfiles)
	if err != nil && !os.IsNotExist(err) {
		ui.Warn(fmt.Sprintf("Could not read dotfiles agents: %v", err))
	}
	agentStatus := fmt.Sprintf("%d local / %d dotfiles", len(localAgentsList), len(dotfileAgentsList))
	if len(localAgentsList) == len(dotfileAgentsList) {
		ui.Success(fmt.Sprintf("Agents: %s", agentStatus))
	} else {
		ui.Warn(fmt.Sprintf("Agents: %s (out of sync)", agentStatus))
	}

	// MCP server counts
	fmt.Println()
	ui.SectionHeader("MCP Server Counts")
	localMCP, localErr := mcp.Load(mcpLocal)
	dotfilesMCP, dotErr := mcp.Load(mcpDotfiles)
	localCount := 0
	dotCount := 0
	if localErr == nil {
		localCount = len(localMCP.Servers)
	}
	if dotErr == nil {
		dotCount = len(dotfilesMCP.Servers)
	}
	mcpStatus := fmt.Sprintf("%d local / %d dotfiles", localCount, dotCount)
	if localCount == dotCount {
		ui.Success(fmt.Sprintf("MCP Servers: %s", mcpStatus))
	} else {
		ui.Warn(fmt.Sprintf("MCP Servers: %s (out of sync)", mcpStatus))
	}

	// Git status
	fmt.Println()
	ui.SectionHeader("Git Status")
	gitStatusOutput, err := git.Status(dotfiles)
	if err != nil {
		ui.Warn(fmt.Sprintf("Git: %v", err))
	} else {
		branch, err := git.Branch(dotfiles)
		if err != nil {
			ui.Warn(fmt.Sprintf("Git branch: %v", err))
		} else {
			ui.Info(fmt.Sprintf("Branch: %s", branch))
		}
		ahead, behind, err := git.AheadBehind(dotfiles)
		if err != nil {
			ui.Warn(fmt.Sprintf("Git ahead/behind: %v", err))
		} else {
			if ahead == 0 && behind == 0 {
				ui.Success("Remote: up to date")
			} else {
				ui.Warn(fmt.Sprintf("Remote: %d ahead, %d behind", ahead, behind))
			}
		}
		if gitStatusOutput == "" {
			ui.Success("Working tree: clean")
		} else {
			ui.Warn("Working tree: dirty")
			fmt.Println(gitStatusOutput)
		}
	}

	// Check symlink status for agents and determine untracked
	sl, _ := symlink.Check(agentsLocal)
	if !sl {
		untracked, _ := agents.Untracked(agentsLocal, agentsDotfiles)
		if len(untracked) > 0 {
			fmt.Println()
			ui.Warn(fmt.Sprintf("%d untracked agent(s) - run 'capture --agents' to add to dotfiles:", len(untracked)))
			for _, a := range untracked {
				ui.Item(a)
			}
		}
	}

	return nil
}

func symlinkStatusRow(name, localPath, dotfilesPath string) []string {
	// Check if dotfiles path exists
	if _, err := os.Stat(dotfilesPath); os.IsNotExist(err) {
		return []string{name, ui.MissingMark + " missing in dotfiles", "run 'capture'"}
	}
	// Check if local path is a symlink pointing to dotfiles
	isLink, err := symlink.Check(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{name, ui.WarnMark + " not installed", "run 'install'"}
		}
		return []string{name, ui.ErrorMark + " error", err.Error()}
	}
	if isLink {
		target, _ := os.Readlink(localPath)
		return []string{name, ui.SuccessMark + " symlinked", target}
	}
	// Not a symlink — check if it's a copy that matches dotfiles (copy-mode install)
	info, _ := os.Stat(localPath)
	if info != nil && info.IsDir() {
		if dirsEqual(localPath, dotfilesPath) {
			return []string{name, ui.SuccessMark + " copied (in sync)", "use 'capture' to push changes"}
		}
		return []string{name, ui.WarnMark + " copied (out of sync)", "run 'capture' to update dotfiles"}
	}
	if filesEqual(localPath, dotfilesPath) {
		return []string{name, ui.SuccessMark + " copied (in sync)", "use 'capture' to push changes"}
	}
	return []string{name, ui.WarnMark + " copied (out of sync)", "run 'capture' to update dotfiles"}
}

// filesEqual returns true when two regular files have identical content.
func filesEqual(a, b string) bool {
	fa, err := os.Open(a)
	if err != nil {
		return false
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false
	}
	defer fb.Close()
	const chunk = 32 * 1024
	ba, bb := make([]byte, chunk), make([]byte, chunk)
	for {
		na, ea := io.ReadFull(fa, ba)
		nb, eb := io.ReadFull(fb, bb)
		if !bytes.Equal(ba[:na], bb[:nb]) {
			return false
		}
		if ea == io.EOF && eb == io.EOF {
			return true
		}
		if ea != nil || eb != nil {
			return false
		}
	}
}

// dirsEqual returns true when every file in src exists in dst with identical content.
func dirsEqual(src, dst string) bool {
	entries, err := os.ReadDir(src)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // shallow check — compare top-level files only
		}
		if !filesEqual(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())) {
			return false
		}
	}
	return true
}
