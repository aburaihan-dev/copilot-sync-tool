package cmd

import (
	"fmt"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/config"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage named dotfiles profiles",
	Long: `Profiles let you switch between multiple dotfiles repositories on the same
machine — useful for personal vs. work Copilot configs, or for testing.

Each profile stores a separate dotfiles directory. The active profile is used
by all other commands (install, capture, status, diff, etc.) when no
--dotfiles flag is provided.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured profiles",
	RunE:  runProfileList,
}

var profileNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileNew,
}

var profileSwitchCmd = &cobra.Command{
	Use:   "switch [name]",
	Short: "Switch the active profile (interactive if name is omitted)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProfileSwitch,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileDelete,
}

var profileNewDir string

func init() {
	profileNewCmd.Flags().StringVar(&profileNewDir, "dotfiles", "", "Dotfiles directory for this profile")
	profileCmd.AddCommand(profileListCmd, profileNewCmd, profileSwitchCmd, profileDeleteCmd)
	rootCmd.AddCommand(profileCmd)
}

// ── list ──────────────────────────────────────────────────────────────────────

func runProfileList(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ui.Header("Profiles")
	fmt.Println()

	if len(cfg.Profiles) == 0 && cfg.DotfilesDir == "" {
		ui.Info("No profiles configured. Run 'copilot-sync-tool profile new <name>' to create one.")
		return nil
	}

	rows := [][]string{{"Profile", "Dotfiles Dir", "Active"}}

	// Default (legacy) entry
	if cfg.DotfilesDir != "" && cfg.ActiveProfile == "" {
		rows = append(rows, []string{"(default)", cfg.DotfilesDir, ui.SuccessMark})
	} else if cfg.DotfilesDir != "" {
		rows = append(rows, []string{"(default)", cfg.DotfilesDir, ""})
	}

	names := sortedProfileNames(cfg.Profiles)
	for _, name := range names {
		active := ""
		if name == cfg.ActiveProfile {
			active = ui.SuccessMark
		}
		rows = append(rows, []string{name, cfg.Profiles[name].DotfilesDir, active})
	}

	ui.Table(rows)
	return nil
}

// ── new ───────────────────────────────────────────────────────────────────────

func runProfileNew(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Profiles != nil {
		if _, exists := cfg.Profiles[name]; exists {
			return fmt.Errorf("profile %q already exists", name)
		}
	}

	dir := profileNewDir
	if dir == "" {
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Dotfiles directory for profile %q:", name),
			Help:    "Absolute path to the dotfiles repository for this profile.",
		}, &dir, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]config.Profile{}
	}
	cfg.Profiles[name] = config.Profile{DotfilesDir: dir}

	// Offer to switch to the new profile immediately
	var switchNow bool
	survey.AskOne(&survey.Confirm{ //nolint:errcheck
		Message: fmt.Sprintf("Switch to profile %q now?", name),
		Default: true,
	}, &switchNow)
	if switchNow {
		cfg.ActiveProfile = name
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	ui.Success(fmt.Sprintf("Profile %q created (dotfiles: %s)", name, dir))
	if switchNow {
		ui.Success(fmt.Sprintf("Active profile: %s", name))
	}
	return nil
}

// ── switch ───────────────────────────────────────────────────────────────────

func runProfileSwitch(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var target string
	if len(args) == 1 {
		target = args[0]
	} else {
		options := sortedProfileNames(cfg.Profiles)
		if cfg.DotfilesDir != "" {
			options = append([]string{"(default)"}, options...)
		}
		if len(options) == 0 {
			return fmt.Errorf("no profiles configured")
		}
		if err := survey.AskOne(&survey.Select{
			Message: "Switch to profile:",
			Options: options,
		}, &target); err != nil {
			return err
		}
	}

	if target == "(default)" {
		cfg.ActiveProfile = ""
	} else {
		if cfg.Profiles == nil || cfg.Profiles[target].DotfilesDir == "" {
			return fmt.Errorf("profile %q does not exist", target)
		}
		cfg.ActiveProfile = target
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	active := cfg.ActiveDotfilesDir()
	ui.Success(fmt.Sprintf("Switched to profile %q (dotfiles: %s)", target, active))
	return nil
}

// ── delete ───────────────────────────────────────────────────────────────────

func runProfileDelete(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Profiles == nil || cfg.Profiles[name].DotfilesDir == "" {
		return fmt.Errorf("profile %q does not exist", name)
	}

	delete(cfg.Profiles, name)
	if cfg.ActiveProfile == name {
		cfg.ActiveProfile = ""
		ui.Warn("Deleted active profile — reverted to default.")
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	ui.Success(fmt.Sprintf("Profile %q deleted.", name))
	return nil
}

func sortedProfileNames(profiles map[string]config.Profile) []string {
	names := make([]string, 0, len(profiles))
	for k := range profiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
