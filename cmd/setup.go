package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/config"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure copilot-sync-tool for this machine",
	Long: `Interactive first-time setup for copilot-sync-tool.

Steps:
  1. Set (or clone) your dotfiles repository path
  2. Save the path so you never need --dotfiles again
  3. Optionally install the binary to a PATH location for system-wide use
  4. Optionally run 'install' to apply Copilot config immediately

The saved config lives at:
  Windows: %APPDATA%\copilot-sync-tool\config.json
  macOS/Linux: ~/.config/copilot-sync-tool/config.json`,
	Example: `  # First-time setup on a new machine
  copilot-sync-tool setup

  # Re-run setup to change the dotfiles repo or reinstall the binary
  copilot-sync-tool setup`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(_ *cobra.Command, _ []string) error {
	ui.Header("copilot-sync-tool setup")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	// ── Step 1: Dotfiles path ────────────────────────────────────────────────
	ui.SectionHeader("Dotfiles Repository")

	// Load any previously saved config as default
	saved, _ := config.Load()
	defaultPath := saved.DotfilesDir
	if defaultPath == "" {
		defaultPath = filepath.Join(home, "copilot-dotfiles")
	}

	var dotfilesInput string
	if err := survey.AskOne(&survey.Input{
		Message: "Path to your dotfiles repo (or a git clone URL):",
		Default: defaultPath,
		Help:    "Enter a local directory path or a git URL (https:// or git@). The directory will be created if it does not exist.",
	}, &dotfilesInput, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	dotfilesInput = strings.TrimSpace(dotfilesInput)

	// ── Step 2: Clone if a URL was given ─────────────────────────────────────
	resolvedPath := dotfilesInput
	if isGitURL(dotfilesInput) {
		var cloneDest string
		if err := survey.AskOne(&survey.Input{
			Message: "Clone into which local directory?",
			Default: defaultPath,
		}, &cloneDest, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		cloneDest = strings.TrimSpace(cloneDest)

		fmt.Printf("%s Cloning %s → %s\n", ui.ActionMark, dotfilesInput, cloneDest)
		if err := gitClone(dotfilesInput, cloneDest); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
		ui.Success("Cloned successfully")
		resolvedPath = cloneDest
	} else {
		// Ensure the directory exists
		if err := os.MkdirAll(resolvedPath, 0o755); err != nil {
			return fmt.Errorf("cannot create directory %s: %w", resolvedPath, err)
		}
	}

	// Expand ~ in paths entered manually
	if strings.HasPrefix(resolvedPath, "~/") || resolvedPath == "~" {
		resolvedPath = filepath.Join(home, resolvedPath[2:])
	}
	resolvedPath, _ = filepath.Abs(resolvedPath)

	// ── Step 3: Save config ───────────────────────────────────────────────────
	cfg := &config.ToolConfig{DotfilesDir: resolvedPath}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	ui.Success(fmt.Sprintf("Dotfiles path saved: %s", resolvedPath))
	fmt.Printf("  %s Config file: %s\n", ui.ItemMark, config.FilePath())

	// ── Step 4: Optional binary install ──────────────────────────────────────
	ui.SectionHeader("Install Binary to PATH")

	installBinTarget, displayPath := binaryInstallTarget()
	fmt.Printf("  Target: %s\n\n", displayPath)

	var doInstallBin bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Install copilot-sync-tool binary to " + displayPath + "?",
		Default: true,
	}, &doInstallBin); err != nil {
		return err
	}

	if doInstallBin {
		if err := installBinary(installBinTarget); err != nil {
			ui.Warn(fmt.Sprintf("Binary install failed: %s", err))
			ui.Warn("You can manually copy the binary to a directory in your PATH.")
		} else {
			ui.Success(fmt.Sprintf("Binary installed to %s", installBinTarget))
			printPathHint(installBinTarget)
		}
	}

	// ── Step 5: Optional immediate install ───────────────────────────────────
	ui.SectionHeader("Apply Config to This Machine")

	var doApply bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Run 'install' now to apply Copilot config from your dotfiles?",
		Default: true,
	}, &doApply); err != nil {
		return err
	}

	if doApply {
		fmt.Println()
		// Set global dotfiles flag and delegate to installCmd
		dotfilesDir = resolvedPath
		return installCmd.RunE(installCmd, nil)
	}

	fmt.Println()
	ui.Success("Setup complete! Run 'copilot-sync-tool install' to apply config.")
	return nil
}

// isGitURL returns true if s looks like a git remote URL.
func isGitURL(s string) bool {
	return strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "git@") ||
		strings.HasPrefix(s, "ssh://")
}

// gitClone runs git clone url dest.
func gitClone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound {
			return fmt.Errorf("git is not installed or not in PATH\n  → Install git from https://git-scm.com/downloads")
		}
		return err
	}
	return nil
}

// binaryInstallTarget returns (absolute target path, display string) for the binary.
//
//   - Linux/macOS: ~/.local/bin/copilot-sync-tool
//   - Windows:     %LOCALAPPDATA%\Programs\copilot-sync-tool\copilot-sync-tool.exe
func binaryInstallTarget() (string, string) {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		p := filepath.Join(localAppData, "Programs", "copilot-sync-tool", "copilot-sync-tool.exe")
		return p, p
	default:
		p := filepath.Join(home, ".local", "bin", "copilot-sync-tool")
		return p, p
	}
}

// installBinary copies the running executable to dest.
func installBinary(dest string) error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate current executable: %w", err)
	}
	// Resolve symlinks so we copy the actual binary
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", filepath.Dir(dest), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open source binary: %w", err)
	}
	defer in.Close()

	// Write to a temp file first, then rename (atomic on most systems)
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", tmp, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("copy failed: %w", err)
	}
	out.Close()

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cannot install binary to %s: %w", dest, err)
	}
	return nil
}

// printPathHint prints a PATH setup hint if the binary directory is not already in PATH.
func printPathHint(binaryPath string) {
	binDir := filepath.Dir(binaryPath)
	pathEnv := os.Getenv("PATH")
	if strings.Contains(pathEnv, binDir) {
		return // already in PATH
	}

	fmt.Println()
	ui.Warn("Add the following to your shell profile so the binary is found in PATH:")
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("\n  [System.Environment]::SetEnvironmentVariable('PATH', $env:PATH + ';%s', 'User')\n\n", binDir)
		fmt.Println("  Or: Settings → System → About → Advanced system settings → Environment Variables")
	default:
		fmt.Printf("\n  export PATH=\"%s:$PATH\"\n\n", binDir)
		fmt.Println("  Add the above line to ~/.bashrc, ~/.zshrc, or ~/.profile.")
	}
}
