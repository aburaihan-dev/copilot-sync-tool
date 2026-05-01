package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/mcp"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/platform"
	"github.com/aburaihan-dev/copilot-sync-tool/internal/ui"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers in your dotfiles config",
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all MCP servers in the dotfiles config",
	RunE:  runMCPList,
}

var mcpAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add an MCP server to the dotfiles config",
	Long: `Adds an MCP server definition to your dotfiles mcp-config.<platform>.json.

If a name matching a built-in registry entry is provided, the server definition
is pre-filled. Otherwise you can enter the command and arguments interactively.

The server is added to the dotfiles config for the current platform. Run
'copilot-sync-tool capture --mcp' to commit and push.`,
	Example: `  # Add from built-in registry
  copilot-sync-tool mcp add github

  # Add interactively
  copilot-sync-tool mcp add

  # Add for all platforms
  copilot-sync-tool mcp add github --all-platforms`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMCPAdd,
}

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove an MCP server from the dotfiles config",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runMCPRemove,
}

var (
	mcpAddAllPlatforms bool
)

func init() {
	mcpAddCmd.Flags().BoolVar(&mcpAddAllPlatforms, "all-platforms", false, "Add the server to all three platform configs (windows, macos, linux)")
	mcpCmd.AddCommand(mcpListCmd, mcpAddCmd, mcpRemoveCmd)
	rootCmd.AddCommand(mcpCmd)
}

// ── Built-in registry ────────────────────────────────────────────────────────

type mcpRegistryEntry struct {
	Description string
	Definition  map[string]interface{}
}

var mcpRegistry = map[string]mcpRegistryEntry{
	"github": {
		Description: "GitHub MCP server (official)",
		Definition: map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", "@modelcontextprotocol/server-github"},
			"env":     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"},
		},
	},
	"filesystem": {
		Description: "Local filesystem MCP server",
		Definition: map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
		},
	},
	"fetch": {
		Description: "HTTP fetch MCP server",
		Definition: map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", "@modelcontextprotocol/server-fetch"},
		},
	},
	"memory": {
		Description: "In-memory knowledge graph MCP server",
		Definition: map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", "@modelcontextprotocol/server-memory"},
		},
	},
	"sequential-thinking": {
		Description: "Sequential thinking MCP server",
		Definition: map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
		},
	},
}

// ── list ──────────────────────────────────────────────────────────────────────

func runMCPList(_ *cobra.Command, _ []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	plat := platform.Current()
	mcpPath := platform.DotfilesMCPConfig(dotfiles, plat)

	cfg, err := mcp.Load(mcpPath)
	if err != nil {
		return fmt.Errorf("loading MCP config: %w", err)
	}

	ui.Header(fmt.Sprintf("MCP Servers (%s)", plat))
	fmt.Println()

	if len(cfg.Servers) == 0 {
		ui.Info("No servers configured.")
		return nil
	}

	names := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := [][]string{{"Server", "Definition (truncated)"}}
	for _, name := range names {
		raw := cfg.Servers[name]
		preview := strings.ReplaceAll(string(raw), "\n", " ")
		if len(preview) > 60 {
			preview = preview[:60] + "…"
		}
		rows = append(rows, []string{name, preview})
	}
	ui.Table(rows)
	return nil
}

// ── add ───────────────────────────────────────────────────────────────────────

func runMCPAdd(_ *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	var serverName string

	if len(args) == 1 {
		serverName = args[0]
	}

	// If name given and in registry, use it; otherwise prompt
	var rawDef json.RawMessage

	if serverName != "" {
		if entry, ok := mcpRegistry[serverName]; ok {
			ui.Info(fmt.Sprintf("Using built-in definition for %q (%s)", serverName, entry.Description))
			rawDef, err = json.Marshal(entry.Definition)
			if err != nil {
				return err
			}
		}
	}

	if serverName == "" {
		// Offer registry choices + "custom"
		options := []string{}
		for name, entry := range mcpRegistry {
			options = append(options, fmt.Sprintf("%s — %s", name, entry.Description))
		}
		sort.Strings(options)
		options = append(options, "custom (enter manually)")

		var chosen string
		if err := survey.AskOne(&survey.Select{
			Message: "Select MCP server to add:",
			Options: options,
		}, &chosen); err != nil {
			return err
		}

		if chosen != "custom (enter manually)" {
			serverName = strings.SplitN(chosen, " — ", 2)[0]
			entry := mcpRegistry[serverName]
			rawDef, err = json.Marshal(entry.Definition)
			if err != nil {
				return err
			}
		}
	}

	if rawDef == nil {
		// Custom: prompt for name + JSON definition
		if serverName == "" {
			if err := survey.AskOne(&survey.Input{
				Message: "Server name:",
			}, &serverName, survey.WithValidator(survey.Required)); err != nil {
				return err
			}
		}
		var defStr string
		if err := survey.AskOne(&survey.Multiline{
			Message: "Server definition (JSON, e.g. {\"command\":\"npx\",\"args\":[...]} ):",
			Help:    "Paste a valid JSON object. It will be validated before saving.",
		}, &defStr, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		if !json.Valid([]byte(defStr)) {
			return fmt.Errorf("invalid JSON definition")
		}
		rawDef = json.RawMessage(defStr)
	}

	platforms := []string{platform.Current()}
	if mcpAddAllPlatforms {
		platforms = []string{"windows", "macos", "linux"}
	}

	for _, plat := range platforms {
		mcpPath := platform.DotfilesMCPConfig(dotfiles, plat)
		cfg, err := mcp.Load(mcpPath)
		if err != nil {
			if !isNotExist(mcpPath) {
				return fmt.Errorf("loading MCP config for %s: %w", plat, err)
			}
			cfg = mcp.NewConfig()
		}
		if cfg.Servers == nil {
			cfg.Servers = map[string]json.RawMessage{}
		}
		cfg.Servers[serverName] = rawDef
		if err := mcp.Save(cfg, mcpPath); err != nil {
			return fmt.Errorf("saving MCP config for %s: %w", plat, err)
		}
		ui.Success(fmt.Sprintf("Added %q to mcp-config.%s.json", serverName, plat))
	}

	ui.Info("Run 'copilot-sync-tool capture --mcp' to commit and push.")
	return nil
}

// ── remove ────────────────────────────────────────────────────────────────────

func runMCPRemove(_ *cobra.Command, args []string) error {
	dotfiles, err := GetDotfilesDir()
	if err != nil {
		return err
	}

	plat := platform.Current()
	mcpPath := platform.DotfilesMCPConfig(dotfiles, plat)

	cfg, err := mcp.Load(mcpPath)
	if err != nil {
		return fmt.Errorf("loading MCP config: %w", err)
	}

	var serverName string
	if len(args) == 1 {
		serverName = args[0]
	} else {
		names := make([]string, 0, len(cfg.Servers))
		for name := range cfg.Servers {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) == 0 {
			ui.Info("No servers to remove.")
			return nil
		}
		if err := survey.AskOne(&survey.Select{
			Message: "Select server to remove:",
			Options: names,
		}, &serverName); err != nil {
			return err
		}
	}

	if _, ok := cfg.Servers[serverName]; !ok {
		return fmt.Errorf("server %q not found in config", serverName)
	}

	var confirm bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("Remove %q from mcp-config.%s.json?", serverName, plat),
		Default: false,
	}, &confirm); err != nil {
		return err
	}
	if !confirm {
		ui.Warn("Cancelled.")
		return nil
	}

	delete(cfg.Servers, serverName)
	if err := mcp.Save(cfg, mcpPath); err != nil {
		return fmt.Errorf("saving MCP config: %w", err)
	}
	ui.Success(fmt.Sprintf("Removed %q from mcp-config.%s.json", serverName, plat))
	ui.Info("Run 'copilot-sync-tool capture --mcp' to commit and push.")
	return nil
}

func isNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
