package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/config"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var dotfilesDir string

var rootCmd = &cobra.Command{
	Use:     "copilot-sync-tool",
	Version: Version,
	Short:   "Sync GitHub Copilot CLI config across devices",
	Long: `copilot-sync-tool manages GitHub Copilot CLI configuration (agents, MCP servers,
settings, and instructions) as dotfiles, enabling cross-device sync across
macOS, Linux, and Windows.

Managed files:
  ~/.copilot/agents/               → dotfiles/copilot/agents/
  ~/.copilot/mcp-config.json       → dotfiles/copilot/mcp-config.<platform>.json
  ~/.copilot/settings.json         → dotfiles/copilot/settings.json
  ~/.copilot/copilot-instructions.md → dotfiles/copilot/copilot-instructions.md

Typical workflow:
  0. Run 'setup'    on a new machine to configure the dotfiles path once
  1. Run 'status'   to see what is in sync
  2. Run 'capture'  on the machine with the latest config (commits + pushes)
  3. Run 'install'  on every other machine to pull and apply the config
  4. Run 'merge'    when local and dotfiles MCP configs have diverged

Dotfiles location (resolved in order):
  --dotfiles flag  →  $COPILOT_DOTFILES_DIR  →  saved config  →  ./  (if copilot/ present)  →  ~/copilot-dotfiles`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dotfilesDir, "dotfiles", "", "Path to dotfiles repo (default: ~/copilot-dotfiles or $COPILOT_DOTFILES_DIR)")
}

// isDotfilesDir returns true if dir looks like a copilot-dotfiles repo
// (i.e. contains a copilot/ subdirectory).
func isDotfilesDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "copilot"))
	return err == nil && info.IsDir()
}

// GetDotfilesDir resolves the dotfiles directory from, in order:
//  1. --dotfiles flag
//  2. COPILOT_DOTFILES_DIR env var
//  3. Saved config (~/.config/copilot-sync-tool/config.json)
//  4. Current working directory (if it contains a copilot/ folder)
//  5. ~/copilot-dotfiles (default)
func GetDotfilesDir() (string, error) {
	if dotfilesDir != "" {
		return dotfilesDir, nil
	}
	if env := os.Getenv("COPILOT_DOTFILES_DIR"); env != "" {
		return env, nil
	}
	if cfg, err := config.Load(); err == nil && cfg.ActiveDotfilesDir() != "" {
		return cfg.ActiveDotfilesDir(), nil
	}
	if cwd, err := os.Getwd(); err == nil && isDotfilesDir(cwd) {
		return cwd, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, "copilot-dotfiles"), nil
}
