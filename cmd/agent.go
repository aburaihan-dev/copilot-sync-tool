package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage Copilot agent files",
}

var agentNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Scaffold a new agent definition file interactively",
	Long: `Creates a new .agent.md file in the dotfiles agents directory using an
interactive prompt. The file is ready to use immediately — no manual editing
of frontmatter required.`,
	Example: `  # Create a new agent interactively
  copilot-sync-tool agent new

  # Create and immediately capture to dotfiles
  copilot-sync-tool agent new --capture`,
	RunE: runAgentNew,
}

var agentNewCapture bool

func init() {
	agentNewCmd.Flags().BoolVar(&agentNewCapture, "capture", false, "Copy the new agent to the dotfiles repo immediately after creation")
	agentCmd.AddCommand(agentNewCmd)
	rootCmd.AddCommand(agentCmd)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func runAgentNew(_ *cobra.Command, _ []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	ui.Header("New Agent")
	fmt.Println()

	var name string
	if err := survey.AskOne(&survey.Input{
		Message: "Agent name (e.g. 'My Reviewer'):",
		Help:    "This becomes the display name shown in the Copilot agent picker.",
	}, &name, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	var description string
	if err := survey.AskOne(&survey.Input{
		Message: "Short description:",
		Help:    "One sentence describing what this agent does.",
	}, &description); err != nil {
		return err
	}

	modelOptions := []string{"gpt-4.1", "gpt-4o", "claude-sonnet-4-5", "claude-opus-4-5", "o3", "gemini-2.5-pro", "(leave blank — use workspace default)"}
	var model string
	if err := survey.AskOne(&survey.Select{
		Message: "Model:",
		Options: modelOptions,
		Default: "(leave blank — use workspace default)",
	}, &model); err != nil {
		return err
	}
	if strings.HasPrefix(model, "(") {
		model = ""
	}

	var instructions string
	if err := survey.AskOne(&survey.Multiline{
		Message: "System instructions (leave blank to fill in later):",
	}, &instructions); err != nil {
		return err
	}

	// Build filename from name slug
	slug := slugRe.ReplaceAllString(strings.ToLower(name), "-")
	slug = strings.Trim(slug, "-")
	filename := slug + ".agent.md"

	// Build frontmatter
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	if description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", description))
	}
	if model != "" {
		sb.WriteString(fmt.Sprintf("model: %s\n", model))
	}
	sb.WriteString("tools: []\n")
	sb.WriteString("---\n\n")
	if instructions != "" {
		sb.WriteString(instructions + "\n")
	} else {
		sb.WriteString("<!-- Add your system instructions here -->\n")
	}

	// Determine destination
	agentsDir := platform.DotfilesAgentsDir(dotfiles)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("cannot create agents directory: %w", err)
	}

	dest := filepath.Join(agentsDir, filename)
	if _, err := os.Stat(dest); err == nil {
		var overwrite bool
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("%s already exists. Overwrite?", filename),
			Default: false,
		}, &overwrite); err != nil {
			return err
		}
		if !overwrite {
			ui.Warn("Cancelled.")
			return nil
		}
	}

	if err := os.WriteFile(dest, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("writing agent file: %w", err)
	}
	ui.Success(fmt.Sprintf("Created %s", dest))

	if agentNewCapture {
		// Also copy to local Copilot agents dir so it's immediately active
		localAgents, err := platform.AgentsDir()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(localAgents, 0755); err != nil {
			return fmt.Errorf("cannot create local agents dir: %w", err)
		}
		localDest := filepath.Join(localAgents, filename)
		data, _ := os.ReadFile(dest)
		if err := os.WriteFile(localDest, data, 0644); err != nil {
			ui.Warn(fmt.Sprintf("Could not copy to local agents dir: %v", err))
		} else {
			ui.Success(fmt.Sprintf("Copied to %s", localDest))
		}
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("File: %s", dest))
	ui.Info("Run 'copilot-sync-tool capture --agents' to push to all devices.")
	return nil
}
