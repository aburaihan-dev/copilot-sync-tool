package platform

import (
	"encoding/json"
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

// CopilotDir returns the GitHub Copilot CLI config directory: ~/.copilot on every platform.
//
// ponytail: earlier versions of this tool guessed OS-specific GUI-app locations
// (%APPDATA%\GitHub Copilot, ~/Library/Application Support/GitHub Copilot, XDG config dir).
// Verified against a live Copilot CLI install: the CLI always uses ~/.copilot — those other
// directories don't exist for the CLI (and on Windows, a %APPDATA%\GitHub Copilot dir left
// behind by this tool's own earlier runs was a stale artifact, not real CLI state).
func CopilotDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".copilot"), nil
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

// DotfilesSkillsDir returns <dotfiles>/copilot/skills
func DotfilesSkillsDir(dotfiles string) string {
	return filepath.Join(dotfiles, "copilot", "skills")
}

// LocalSkillDirectories reads the "skillDirectories" array out of the local settings.json.
// Returns nil (not an error) if settings.json is missing or has no such key.
func LocalSkillDirectories() ([]string, error) {
	settingsPath, err := SettingsFile()
	if err != nil {
		return nil, err
	}
	return SkillDirectoriesFrom(settingsPath)
}

// SkillDirectoriesFrom reads the "skillDirectories" array out of the settings.json at path.
// Returns nil (not an error) if the file is missing or has no such key.
func SkillDirectoriesFrom(settingsPath string) ([]string, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var parsed struct {
		SkillDirectories []string `json:"skillDirectories"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing settings.json: %w", err)
	}
	return parsed.SkillDirectories, nil
}

// Hostname returns the machine hostname (best effort).
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

