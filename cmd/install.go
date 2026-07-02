package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/agents"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/git"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/mcp"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/skills"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/symlink"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var (
	installMCP         bool
	installAgents      bool
	installSkills      bool
	installSettings    bool
	installInstructions bool
	installInteractive bool
	installAll         bool
	installCopy        bool
	installPull        bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Copilot config from dotfiles via symlinks or copy",
	Long: `Pulls config from the dotfiles repo and installs it on this device.

By default an interactive picker lets you choose which components and individual
items (MCP servers, agents) to install. Pass --all to skip the picker and
install everything at once.

Files are installed as symlinks so future 'git pull' runs in the dotfiles repo
immediately reflect on this machine. Use --copy to copy files instead (useful
on Windows without Developer Mode, or for selective overrides).

On Windows, symlink creation requires Developer Mode (Settings → System → For
Developers) or Administrator privileges. If neither is available the tool
automatically falls back to --copy mode and prints a warning.

Managed files:
  mcp-config.json         ← dotfiles/copilot/mcp-config.<platform>.json
  settings.json           ← dotfiles/copilot/settings.json
  agents/                 ← dotfiles/copilot/agents/
  skills/                 ← dotfiles/copilot/skills/ (installed at paths from settings.json "skillDirectories")
  copilot-instructions.md ← dotfiles/copilot/copilot-instructions.md`,
	Example: `  # Interactive picker (default — select components and individual items)
  copilot-sync-tool install

  # Install everything without prompting
  copilot-sync-tool install --all

  # Copy files instead of symlinking
  copilot-sync-tool install --copy

  # Install only MCP config, skip git pull
  copilot-sync-tool install --mcp --no-pull

  # Install from a custom dotfiles location
  copilot-sync-tool install --dotfiles ~/my-dotfiles`,
	RunE: runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&installMCP, "mcp", false, "Install only mcp-config.json")
	installCmd.Flags().BoolVar(&installAgents, "agents", false, "Install only agents/")
	installCmd.Flags().BoolVar(&installSkills, "skills", false, "Install only skills/")
	installCmd.Flags().BoolVar(&installSettings, "settings", false, "Install only settings.json")
	installCmd.Flags().BoolVar(&installInstructions, "instructions", false, "Install only copilot-instructions.md")
	installCmd.Flags().BoolVar(&installAll, "all", false, "Install all components without prompting")
	installCmd.Flags().BoolVarP(&installInteractive, "interactive", "i", false, "Interactive multi-select mode (default when no component flag is set)")
	installCmd.Flags().MarkHidden("interactive") //nolint kept for backward compat
	installCmd.Flags().BoolVar(&installCopy, "copy", false, "Copy files instead of symlinking (auto-enabled on Windows when symlinks are unavailable)")
	installCmd.Flags().BoolVar(&installPull, "pull", true, "Git pull before installing (disable with --no-pull)")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(dotfiles); os.IsNotExist(err) {
		return fmt.Errorf("dotfiles directory does not exist: %s", dotfiles)
	}

	plat := platform.Current()

	ui.Header("Installing Copilot Config")
	fmt.Println()

	// Auto-detect symlink support on Windows and fall back to copy mode.
	if !installCopy && !symlink.CanCreate(dotfiles) {
		ui.Warn("Symlinks are not available on this system (Windows may require Developer Mode or Administrator).")
		ui.Warn("Automatically switching to --copy mode. Run with --copy to suppress this warning.")
		fmt.Println()
		installCopy = true
	}

	// Git pull
	if installPull {
		ui.Action("Pulling from git remote...")
		if err := git.Pull(dotfiles); err != nil {
			ui.Warn(fmt.Sprintf("Git pull: %v", err))
		} else {
			ui.Success("Git pull complete")
		}
		fmt.Println()
	}

	// Specific component flags were passed — install just those, no prompt.
	hasComponentFlag := installMCP || installAgents || installSkills || installSettings || installInstructions

	if !hasComponentFlag && !installAll && !installInteractive {
		// Default: interactive picker
		return runInstallInteractive(dotfiles, plat)
	}
	if installInteractive {
		return runInstallInteractive(dotfiles, plat)
	}

	// Ensure copilot dir exists
	copilotDir, err := platform.CopilotDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		return fmt.Errorf("cannot create copilot dir: %w", err)
	}

	installEverything := installAll || (!installMCP && !installAgents && !installSkills && !installSettings && !installInstructions)

	// Install MCP
	if installEverything || installMCP {
		ui.SectionHeader("MCP Config")
		if err := installMCPConfig(dotfiles, plat); err != nil {
			ui.Error(fmt.Sprintf("MCP install: %v", err))
		}
		fmt.Println()
	}

	// Install Agents
	if installEverything || installAgents {
		ui.SectionHeader("Agents")
		if err := installAgentsDir(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Agents install: %v", err))
		}
		fmt.Println()
	}

	// Install Skills
	if installEverything || installSkills {
		ui.SectionHeader("Skills")
		if err := installSkillsDirs(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Skills install: %v", err))
		}
		fmt.Println()
	}

	// Install Settings
	if installEverything || installSettings {
		ui.SectionHeader("Settings")
		if err := doInstallSettings(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Settings install: %v", err))
		}
		fmt.Println()
	}

	// Install Instructions
	if installEverything || installInstructions {
		ui.SectionHeader("Instructions")
		if err := installInstructionsFile(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Instructions install: %v", err))
		}
	}

	return nil
}

func installMCPConfig(dotfiles, plat string) error {
	src := platform.DotfilesMCPConfig(dotfiles, plat)
	dst, err := platform.MCPConfigFile()
	if err != nil {
		return err
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		ui.Warn(fmt.Sprintf("No dotfiles MCP config for platform %s", plat))
		return nil
	}

	if installCopy {
		return copyWithBackup(src, dst)
	}
	return symlinkWithBackup(src, dst)
}

func installAgentsDir(dotfiles string) error {
	src := platform.DotfilesAgentsDir(dotfiles)
	dst, err := platform.AgentsDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		ui.Warn("No agents directory in dotfiles")
		return nil
	}

	// Check for untracked agents
	isLink, _ := symlink.Check(dst)
	if !isLink {
		untracked, err := agents.Untracked(dst, src)
		if err == nil && len(untracked) > 0 {
			ui.Warn(fmt.Sprintf("%d untracked agent(s) found - run 'capture --agents' first:", len(untracked)))
			for _, a := range untracked {
				ui.Item(a)
			}
			return nil
		}
	}

	if installCopy {
		return copyDirContents(src, dst)
	}
	return symlinkWithBackup(src, dst)
}

// installSkillsDirs installs each skill directory tracked in dotfiles/copilot/skills/ to the
// local path recorded for it in settings.json's "skillDirectories" (dotfiles settings.json is
// authoritative; falls back to the local settings.json if dotfiles has none yet).
func installSkillsDirs(dotfiles string) error {
	skillsDotfiles := platform.DotfilesSkillsDir(dotfiles)
	names, err := skills.List(skillsDotfiles)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		ui.Info("No skills found in dotfiles")
		return nil
	}

	targets, err := platform.SkillDirectoriesFrom(platform.DotfilesSettingsFile(dotfiles))
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		targets, err = platform.LocalSkillDirectories()
		if err != nil {
			return err
		}
	}
	targetByName := make(map[string]string, len(targets))
	for _, t := range targets {
		targetByName[filepath.Base(t)] = t
	}

	for _, name := range names {
		src := filepath.Join(skillsDotfiles, name)
		dst, ok := targetByName[name]
		if !ok {
			// ponytail: no recorded target path for this skill, skip rather than guess a location.
			ui.Skip(fmt.Sprintf("%s — no skillDirectories entry for it, add one to settings.json", name))
			continue
		}
		if installCopy {
			if err := skills.CopyDir(src, dst); err != nil {
				ui.Error(fmt.Sprintf("%s: %v", name, err))
			} else {
				ui.Success(fmt.Sprintf("Copied skill: %s", name))
			}
			continue
		}
		if err := symlinkWithBackup(src, dst); err != nil {
			ui.Error(fmt.Sprintf("%s: %v", name, err))
		}
	}
	return nil
}

func doInstallSettings(dotfiles string) error {
	src := platform.DotfilesSettingsFile(dotfiles)
	dst, err := platform.SettingsFile()
	if err != nil {
		return err
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		ui.Warn("No settings.json in dotfiles")
		return nil
	}

	if installCopy {
		return copyWithBackup(src, dst)
	}
	return symlinkWithBackup(src, dst)
}

func installInstructionsFile(dotfiles string) error {
	src := platform.DotfilesInstructionsFile(dotfiles)
	dst, err := platform.InstructionsFile()
	if err != nil {
		return err
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		ui.Info("No copilot-instructions.md in dotfiles - skipping")
		return nil
	}

	if installCopy {
		return copyWithBackup(src, dst)
	}
	return symlinkWithBackup(src, dst)
}

func symlinkWithBackup(src, dst string) error {
	// Check if already symlinked correctly
	if isLink, err := symlink.Check(dst); err == nil && isLink {
		target, _ := os.Readlink(dst)
		if target == src {
			ui.Skip(fmt.Sprintf("%s already symlinked", filepath.Base(dst)))
			return nil
		}
	}

	// Backup existing real file/dir
	if _, err := os.Lstat(dst); err == nil {
		bak := dst + ".bak"
		ui.Action(fmt.Sprintf("Backing up %s → %s", dst, bak))
		if err := os.Rename(dst, bak); err != nil {
			return fmt.Errorf("backup failed for %s: %w", dst, err)
		}
	}

	if err := symlink.Create(src, dst); err != nil {
		return fmt.Errorf("symlink %s → %s: %w", dst, src, err)
	}
	ui.Success(fmt.Sprintf("Symlinked: %s → %s", dst, src))
	return nil
}

func copyWithBackup(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		bak := dst + ".bak"
		ui.Action(fmt.Sprintf("Backing up %s → %s", dst, bak))
		if err := os.Rename(dst, bak); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Copied: %s → %s", src, dst))
	return nil
}

func copyDirContents(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		srcFile := filepath.Join(src, e.Name())
		dstFile := filepath.Join(dst, e.Name())
		data, err := os.ReadFile(srcFile)
		if err != nil {
			ui.Warn(fmt.Sprintf("Could not read %s: %v", e.Name(), err))
			continue
		}
		if err := os.WriteFile(dstFile, data, 0644); err != nil {
			ui.Warn(fmt.Sprintf("Could not write %s: %v", e.Name(), err))
			continue
		}
		ui.Success(fmt.Sprintf("Copied: %s", e.Name()))
	}
	return nil
}

// runInstallInteractive handles interactive multi-select install
func runInstallInteractive(dotfiles, plat string) error {
	componentOptions := []string{"Settings", "MCP Servers", "Agents", "Skills", "Instructions"}
	var selectedComponents []string
	prompt := &survey.MultiSelect{
		Message: "Which components to install?",
		Options: componentOptions,
		Default: componentOptions,
	}
	if err := survey.AskOne(prompt, &selectedComponents); err != nil {
		return fmt.Errorf("prompt error: %w", err)
	}

	selected := map[string]bool{}
	for _, c := range selectedComponents {
		selected[c] = true
	}

	copilotDir, err := platform.CopilotDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		return fmt.Errorf("cannot create copilot dir: %w", err)
	}

	if selected["Settings"] {
		ui.SectionHeader("Settings")
		if err := doInstallSettings(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Settings: %v", err))
		}
	}

	if selected["Instructions"] {
		ui.SectionHeader("Instructions")
		if err := installInstructionsFile(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Instructions: %v", err))
		}
	}

	if selected["MCP Servers"] {
		ui.SectionHeader("MCP Servers")
		if err := runInteractiveMCPInstall(dotfiles, plat); err != nil {
			ui.Error(fmt.Sprintf("MCP: %v", err))
		}
	}

	if selected["Agents"] {
		ui.SectionHeader("Agents")
		if err := runInteractiveAgentsInstall(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Agents: %v", err))
		}
	}

	if selected["Skills"] {
		// ponytail: reuse the non-interactive path — skill dirs are per-skill trees,
		// not worth a bespoke per-file multi-select like agents get.
		ui.SectionHeader("Skills")
		if err := installSkillsDirs(dotfiles); err != nil {
			ui.Error(fmt.Sprintf("Skills: %v", err))
		}
	}

	return nil
}

func runInteractiveMCPInstall(dotfiles, plat string) error {
	mcpDotfilesPath := platform.DotfilesMCPConfig(dotfiles, plat)
	cfg, err := mcp.Load(mcpDotfilesPath)
	if err != nil {
		return fmt.Errorf("loading dotfiles MCP config: %w", err)
	}

	serverNames := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		serverNames = append(serverNames, name)
	}

	if len(serverNames) == 0 {
		ui.Info("No MCP servers found in dotfiles")
		return nil
	}

	var selectedServers []string
	serverPrompt := &survey.MultiSelect{
		Message: "Select MCP servers to install:",
		Options: serverNames,
		Default: serverNames,
	}
	if err := survey.AskOne(serverPrompt, &selectedServers); err != nil {
		return err
	}

	mcpDst, err := platform.MCPConfigFile()
	if err != nil {
		return err
	}

	if len(selectedServers) == len(serverNames) {
		return symlinkWithBackup(mcpDotfilesPath, mcpDst)
	}

	// Subset selected → write filtered copy
	filtered := &mcp.Config{Servers: map[string]json.RawMessage{}}
	for _, name := range selectedServers {
		filtered.Servers[name] = cfg.Servers[name]
	}

	// Backup existing
	if _, err := os.Lstat(mcpDst); err == nil {
		bak := mcpDst + ".bak"
		ui.Action(fmt.Sprintf("Backing up %s → %s", mcpDst, bak))
		if err := os.Rename(mcpDst, bak); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}

	if err := mcp.Save(filtered, mcpDst); err != nil {
		return fmt.Errorf("writing filtered MCP config: %w", err)
	}
	ui.Success(fmt.Sprintf("Wrote MCP config with %d server(s)", len(selectedServers)))
	return nil
}

func runInteractiveAgentsInstall(dotfiles string) error {
	agentsDotfiles := platform.DotfilesAgentsDir(dotfiles)
	agentsDst, err := platform.AgentsDir()
	if err != nil {
		return err
	}

	allAgents, err := agents.List(agentsDotfiles)
	if err != nil {
		return fmt.Errorf("listing dotfiles agents: %w", err)
	}
	if len(allAgents) == 0 {
		ui.Info("No agents found in dotfiles")
		return nil
	}

	var selectedAgents []string
	agentPrompt := &survey.MultiSelect{
		Message: "Select agents to install:",
		Options: allAgents,
		Default: allAgents,
	}
	if err := survey.AskOne(agentPrompt, &selectedAgents); err != nil {
		return err
	}

	if len(selectedAgents) == len(allAgents) {
		isLink, _ := symlink.Check(agentsDst)
		if !isLink {
			untracked, _ := agents.Untracked(agentsDst, agentsDotfiles)
			if len(untracked) > 0 {
				ui.Warn(fmt.Sprintf("%d untracked agent(s) - run 'capture --agents' first", len(untracked)))
				return nil
			}
		}
		return symlinkWithBackup(agentsDotfiles, agentsDst)
	}

	// Subset → copy selected files
	if err := os.MkdirAll(agentsDst, 0755); err != nil {
		return err
	}
	for _, agentName := range selectedAgents {
		src := filepath.Join(agentsDotfiles, agentName)
		dst := filepath.Join(agentsDst, agentName)
		data, err := os.ReadFile(src)
		if err != nil {
			ui.Warn(fmt.Sprintf("Could not read %s: %v", agentName, err))
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			ui.Warn(fmt.Sprintf("Could not write %s: %v", agentName, err))
			continue
		}
		ui.Success(fmt.Sprintf("Copied agent: %s", agentName))
	}
	return nil
}
