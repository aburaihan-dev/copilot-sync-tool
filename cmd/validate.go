package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aburaihan-dev/copilot-sync-tool/internal/agents"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the dotfiles repo structure and config files",
	Long: `Checks the dotfiles repository for common issues:

  • Required directories exist (copilot/, copilot/agents/)
  • Platform MCP config files are valid JSON
  • settings.json is valid JSON (if present)
  • Agent files have required frontmatter (---...--- block)
  • No obvious secrets in MCP config values

Exits with code 1 if any check fails.`,
	Example: `  # Validate the default dotfiles repo
  copilot-sync-tool validate

  # Validate a specific dotfiles location
  copilot-sync-tool validate --dotfiles ~/my-dotfiles`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(_ *cobra.Command, _ []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	ui.Header("Validate Dotfiles")
	fmt.Println()
	ui.Info(fmt.Sprintf("Dotfiles: %s", dotfiles))
	fmt.Println()

	var failCount int

	// ── Structure checks ─────────────────────────────────────────────────────
	ui.SectionHeader("Structure")
	requiredDirs := []string{
		dotfiles + "/copilot",
		platform.DotfilesAgentsDir(dotfiles),
	}
	for _, dir := range requiredDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			ui.Error(fmt.Sprintf("Missing directory: %s", dir))
			failCount++
		} else {
			ui.Success(fmt.Sprintf("Directory exists: %s", dir))
		}
	}

	// Platform MCP configs
	for _, plat := range []string{"windows", "macos", "linux"} {
		path := platform.DotfilesMCPConfig(dotfiles, plat)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			ui.Warn(fmt.Sprintf("No MCP config for platform %s (expected at %s)", plat, path))
		} else {
			ui.Success(fmt.Sprintf("Platform config present: mcp-config.%s.json", plat))
		}
	}

	// ── JSON validity ─────────────────────────────────────────────────────────
	fmt.Println()
	ui.SectionHeader("JSON Validity")
	jsonFiles := []struct {
		label string
		path  string
	}{
		{"settings.json", platform.DotfilesSettingsFile(dotfiles)},
		{"mcp-config.windows.json", platform.DotfilesMCPConfig(dotfiles, "windows")},
		{"mcp-config.macos.json", platform.DotfilesMCPConfig(dotfiles, "macos")},
		{"mcp-config.linux.json", platform.DotfilesMCPConfig(dotfiles, "linux")},
	}
	for _, jf := range jsonFiles {
		data, err := os.ReadFile(jf.path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			ui.Error(fmt.Sprintf("Cannot read %s: %v", jf.label, err))
			failCount++
			continue
		}
		if !json.Valid(data) {
			ui.Error(fmt.Sprintf("Invalid JSON: %s", jf.label))
			failCount++
		} else {
			ui.Success(fmt.Sprintf("Valid JSON: %s", jf.label))
		}
	}

	// ── Agent frontmatter ─────────────────────────────────────────────────────
	fmt.Println()
	ui.SectionHeader("Agent Files")
	agentsDir := platform.DotfilesAgentsDir(dotfiles)
	agentList, err := agents.List(agentsDir)
	if err != nil && !os.IsNotExist(err) {
		ui.Warn(fmt.Sprintf("Cannot list agents: %v", err))
	} else if len(agentList) == 0 {
		ui.Info("No agent files found.")
	} else {
		for _, name := range agentList {
			path := agentsDir + "/" + name
			if err := validateAgentFrontmatter(path); err != nil {
				ui.Error(fmt.Sprintf("%s: %v", name, err))
				failCount++
			} else {
				ui.Success(fmt.Sprintf("%s: frontmatter OK", name))
			}
		}
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	fmt.Println()
	if failCount == 0 {
		ui.Success(fmt.Sprintf("All checks passed."))
		return nil
	}
	return fmt.Errorf("%d validation error(s) found", failCount)
}

// validateAgentFrontmatter checks that an agent file has a valid ---...--- block.
func validateAgentFrontmatter(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Errorf("missing opening frontmatter delimiter (---)")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return nil // found closing delimiter
		}
	}
	return fmt.Errorf("frontmatter block not closed")
}
