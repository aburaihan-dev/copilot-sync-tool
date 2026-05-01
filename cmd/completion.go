package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate and install shell completion scripts",
}

var completionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell completion for the current shell",
	Long: `Detects your shell and writes a completion script to the appropriate
profile file. After installation you must reload your shell (or open a new
terminal) for completions to take effect.

Supported shells: bash, zsh, fish, powershell`,
	Example: `  # Auto-detect shell and install
  copilot-sync-tool completion install

  # Generate completion script to stdout (for manual setup)
  copilot-sync-tool completion bash
  copilot-sync-tool completion zsh
  copilot-sync-tool completion fish
  copilot-sync-tool completion powershell`,
	RunE: runCompletionInstall,
}

var completionShellFlag string

func init() {
	completionInstallCmd.Flags().StringVar(&completionShellFlag, "shell", "", "Shell type: bash, zsh, fish, or powershell (auto-detected if omitted)")
	completionCmd.AddCommand(completionInstallCmd)

	// Generate-to-stdout sub-commands (standard cobra pattern)
	completionCmd.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenBashCompletion(os.Stdout)
		},
	})
	completionCmd.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenZshCompletion(os.Stdout)
		},
	})
	completionCmd.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenFishCompletion(os.Stdout, true)
		},
	})
	completionCmd.AddCommand(&cobra.Command{
		Use:   "powershell",
		Short: "Generate PowerShell completion script",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		},
	})

	rootCmd.AddCommand(completionCmd)
}

func runCompletionInstall(_ *cobra.Command, _ []string) error {
	ui.Header("Install Shell Completion")
	fmt.Println()

	shell := completionShellFlag
	if shell == "" {
		shell = detectShell()
	}
	if shell == "" {
		return fmt.Errorf("could not detect shell. Use --shell=bash|zsh|fish|powershell")
	}

	ui.Info(fmt.Sprintf("Shell: %s", shell))

	switch shell {
	case "bash":
		return installBashCompletion()
	case "zsh":
		return installZshCompletion()
	case "fish":
		return installFishCompletion()
	case "powershell":
		return installPowerShellCompletion()
	default:
		return fmt.Errorf("unsupported shell %q. Use bash, zsh, fish, or powershell", shell)
	}
}

func detectShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return "zsh"
	case strings.Contains(shell, "fish"):
		return "fish"
	case strings.Contains(shell, "bash"):
		return "bash"
	default:
		return ""
	}
}

func installBashCompletion() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".bash_completion.d")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(dir, "copilot-sync-tool")
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot write completion file: %w", err)
	}
	defer f.Close()
	if err := rootCmd.GenBashCompletion(f); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Written: %s", dest))

	bashrc := filepath.Join(home, ".bashrc")
	sourceLine := fmt.Sprintf("\n# copilot-sync-tool completion\n[[ -d ~/.bash_completion.d ]] && for f in ~/.bash_completion.d/*; do . \"$f\"; done\n")
	appendIfAbsent(bashrc, sourceLine, "copilot-sync-tool completion")
	ui.Info("Reload your shell: source ~/.bashrc")
	return nil
}

func installZshCompletion() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".zsh", "completions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(dir, "_copilot-sync-tool")
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot write completion file: %w", err)
	}
	defer f.Close()
	if err := rootCmd.GenZshCompletion(f); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Written: %s", dest))

	zshrc := filepath.Join(home, ".zshrc")
	sourceLine := fmt.Sprintf("\n# copilot-sync-tool completion\nfpath=(~/.zsh/completions $fpath)\nautoload -Uz compinit && compinit\n")
	appendIfAbsent(zshrc, sourceLine, "copilot-sync-tool completion")
	ui.Info("Reload your shell: source ~/.zshrc")
	return nil
}

func installFishCompletion() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".config", "fish", "completions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(dir, "copilot-sync-tool.fish")
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("cannot write completion file: %w", err)
	}
	defer f.Close()
	if err := rootCmd.GenFishCompletion(f, true); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Written: %s", dest))
	ui.Info("Fish completions are loaded automatically on next shell start.")
	return nil
}

func installPowerShellCompletion() error {
	profilePath, err := powershellProfilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
		return err
	}

	// Write completion to a dedicated file next to the profile
	completionFile := filepath.Join(filepath.Dir(profilePath), "copilot-sync-tool.completion.ps1")
	f, err := os.Create(completionFile)
	if err != nil {
		return fmt.Errorf("cannot write completion file: %w", err)
	}
	defer f.Close()
	if err := rootCmd.GenPowerShellCompletionWithDesc(f); err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Written: %s", completionFile))

	sourceLine := fmt.Sprintf("\n# copilot-sync-tool completion\n. \"%s\"\n", completionFile)
	appendIfAbsent(profilePath, sourceLine, "copilot-sync-tool completion")
	ui.Success(fmt.Sprintf("Profile updated: %s", profilePath))
	ui.Info("Reload your profile: . $PROFILE")
	return nil
}

func powershellProfilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		docs := os.Getenv("USERPROFILE")
		if docs == "" {
			docs = home
		}
		return filepath.Join(docs, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
	}
	return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"), nil
}

// appendIfAbsent appends content to a file if marker string is not already present.
func appendIfAbsent(path, content, marker string) {
	existing, _ := os.ReadFile(path)
	if strings.Contains(string(existing), marker) {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		ui.Warn(fmt.Sprintf("Could not update %s: %v", path, err))
		return
	}
	defer f.Close()
	f.WriteString(content) //nolint:errcheck
}
