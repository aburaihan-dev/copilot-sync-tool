package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Current returns the platform string: "macos", "linux", or "windows".
func Current() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

// HomeDir returns the user's home directory.
func HomeDir() (string, error) {
	return os.UserHomeDir()
}

// CopilotDir returns the platform-specific GitHub Copilot config directory:
//   - Windows: %APPDATA%\GitHub Copilot  (falls back to ~\AppData\Roaming\GitHub Copilot)
//   - macOS:   ~/Library/Application Support/GitHub Copilot
//   - Linux:   ~/.config/github-copilot  (respects $XDG_CONFIG_HOME)
func CopilotDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "GitHub Copilot"), nil
		}
		return filepath.Join(home, "AppData", "Roaming", "GitHub Copilot"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "GitHub Copilot"), nil
	default: // linux and others
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "github-copilot"), nil
		}
		return filepath.Join(home, ".config", "github-copilot"), nil
	}
}

// AgentsDir returns the platform-specific agents directory inside CopilotDir.
func AgentsDir() (string, error) {
	dir, err := CopilotDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agents"), nil
}

// MCPConfigFile returns the platform-specific MCP config file path inside CopilotDir.
func MCPConfigFile() (string, error) {
	dir, err := CopilotDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mcp-config.json"), nil
}

// SettingsFile returns the platform-specific settings.json path inside CopilotDir.
func SettingsFile() (string, error) {
	dir, err := CopilotDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// InstructionsFile returns the platform-specific copilot-instructions.md path inside CopilotDir.
func InstructionsFile() (string, error) {
	dir, err := CopilotDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "copilot-instructions.md"), nil
}

// DotfilesAgentsDir returns <dotfiles>/copilot/agents
func DotfilesAgentsDir(dotfiles string) string {
	return filepath.Join(dotfiles, "copilot", "agents")
}

// DotfilesMCPConfig returns <dotfiles>/copilot/mcp-config.<platform>.json
func DotfilesMCPConfig(dotfiles, plat string) string {
	return filepath.Join(dotfiles, "copilot", fmt.Sprintf("mcp-config.%s.json", plat))
}

// DotfilesSettingsFile returns <dotfiles>/copilot/settings.json
func DotfilesSettingsFile(dotfiles string) string {
	return filepath.Join(dotfiles, "copilot", "settings.json")
}

// DotfilesInstructionsFile returns <dotfiles>/copilot/copilot-instructions.md
func DotfilesInstructionsFile(dotfiles string) string {
	return filepath.Join(dotfiles, "copilot", "copilot-instructions.md")
}

// Hostname returns the machine hostname (best effort).
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

