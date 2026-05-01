package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// ToolConfig holds the persistent configuration for copilot-sync-tool.
type ToolConfig struct {
	DotfilesDir   string             `json:"dotfiles_dir,omitempty"`
	ActiveProfile string             `json:"active_profile,omitempty"`
	Profiles      map[string]Profile `json:"profiles,omitempty"`
}

// Profile holds dotfiles settings for a named profile.
type Profile struct {
	DotfilesDir string `json:"dotfiles_dir"`
}

// ActiveDotfilesDir returns the dotfiles directory for the active profile,
// falling back to the top-level DotfilesDir for backward compatibility.
func (c *ToolConfig) ActiveDotfilesDir() string {
	if c.ActiveProfile != "" && c.Profiles != nil {
		if p, ok := c.Profiles[c.ActiveProfile]; ok && p.DotfilesDir != "" {
			return p.DotfilesDir
		}
	}
	return c.DotfilesDir
}

// configFilePath returns the OS-specific path to the tool's own config file.
//
//   - Windows: %APPDATA%\copilot-sync-tool\config.json
//   - macOS/Linux: ~/.config/copilot-sync-tool/config.json  (respects $XDG_CONFIG_HOME)
func configFilePath() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			dir = filepath.Join(appdata, "copilot-sync-tool")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, "AppData", "Roaming", "copilot-sync-tool")
		}
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			dir = filepath.Join(xdg, "copilot-sync-tool")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, ".config", "copilot-sync-tool")
		}
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the tool config from disk. Returns an empty config if the file does not exist.
func Load() (*ToolConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return &ToolConfig{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ToolConfig{}, nil
	}
	if err != nil {
		return &ToolConfig{}, err
	}
	var cfg ToolConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &ToolConfig{}, err
	}
	return &cfg, nil
}

// Save writes the tool config to disk, creating directories as needed.
func Save(cfg *ToolConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// FilePath returns the resolved config file path (for display in setup output).
func FilePath() string {
	p, _ := configFilePath()
	return p
}
