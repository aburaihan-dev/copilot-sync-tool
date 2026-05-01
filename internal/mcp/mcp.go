package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Config represents a parsed mcp-config.json
type Config struct {
	Servers map[string]json.RawMessage `json:"mcpServers"`
}

// NewConfig returns an empty Config.
func NewConfig() *Config {
	return &Config{Servers: map[string]json.RawMessage{}}
}

// DiffResult holds the result of comparing two configs
type DiffResult struct {
	OnlyLocal    []string
	OnlyDotfiles []string
	Same         []string
	Different    []string
}

// Load reads and parses a mcp-config.json file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.Servers == nil {
		cfg.Servers = map[string]json.RawMessage{}
	}
	return &cfg, nil
}

// Save writes a Config to a file with pretty-print formatting.
func Save(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling MCP config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Clone returns a deep copy of a Config.
func Clone(cfg *Config) *Config {
	c := &Config{Servers: map[string]json.RawMessage{}}
	for k, v := range cfg.Servers {
		cp := make(json.RawMessage, len(v))
		copy(cp, v)
		c.Servers[k] = cp
	}
	return c
}

// Diff computes the difference between local and dotfiles configs.
func Diff(local, dotfiles *Config) DiffResult {
	result := DiffResult{}

	for name, localVal := range local.Servers {
		if dotVal, ok := dotfiles.Servers[name]; !ok {
			result.OnlyLocal = append(result.OnlyLocal, name)
		} else if string(localVal) == string(dotVal) {
			result.Same = append(result.Same, name)
		} else {
			result.Different = append(result.Different, name)
		}
	}

	for name := range dotfiles.Servers {
		if _, ok := local.Servers[name]; !ok {
			result.OnlyDotfiles = append(result.OnlyDotfiles, name)
		}
	}

	sort.Strings(result.OnlyLocal)
	sort.Strings(result.OnlyDotfiles)
	sort.Strings(result.Same)
	sort.Strings(result.Different)

	return result
}

// CaptureInto merges new servers from localPath into dotfilesPath.
// Existing servers in dotfiles are preserved. Returns names of added servers.
func CaptureInto(localPath, dotfilesPath string) ([]string, error) {
	local, err := Load(localPath)
	if err != nil {
		return nil, fmt.Errorf("loading local config: %w", err)
	}

	var dotfiles *Config
	dotfiles, err = Load(dotfilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			dotfiles = NewConfig()
		} else {
			return nil, fmt.Errorf("loading dotfiles config: %w", err)
		}
	}

	var added []string
	for name, val := range local.Servers {
		if _, exists := dotfiles.Servers[name]; !exists {
			dotfiles.Servers[name] = val
			added = append(added, name)
		}
	}

	if len(added) == 0 {
		return nil, nil
	}

	if err := Save(dotfiles, dotfilesPath); err != nil {
		return nil, err
	}
	sort.Strings(added)
	return added, nil
}

// PrettyServer returns the JSON of a single server from the config, pretty-printed.
func PrettyServer(cfg *Config, name string) (string, error) {
	val, ok := cfg.Servers[name]
	if !ok {
		return "", fmt.Errorf("server %q not found", name)
	}
	data, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Filter returns a new Config with only the specified server names.
func Filter(cfg *Config, names []string) *Config {
	filtered := NewConfig()
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	for name, val := range cfg.Servers {
		if nameSet[name] {
			filtered.Servers[name] = val
		}
	}
	return filtered
}
