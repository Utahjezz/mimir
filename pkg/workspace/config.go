package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoCurrentWorkspace is returned when no active workspace has been set.
var ErrNoCurrentWorkspace = errors.New("no current workspace set; run `mimir workspace use <name>` first")

type globalConfig struct {
	CurrentWorkspace string `json:"current_workspace"`
}

// GetCurrentWorkspace reads the active workspace name from the global config file.
// Returns ErrNoCurrentWorkspace if none has been set yet.
func GetCurrentWorkspace() (string, error) {
	path, err := configFilePath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoCurrentWorkspace
	}
	if err != nil {
		return "", fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg globalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("cannot parse config file: %w", err)
	}

	if cfg.CurrentWorkspace == "" {
		return "", ErrNoCurrentWorkspace
	}

	return cfg.CurrentWorkspace, nil
}

// SetCurrentWorkspace writes the active workspace name to the global config file.
func SetCurrentWorkspace(name string) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	data, err := json.MarshalIndent(globalConfig{CurrentWorkspace: name}, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}

	return nil
}

// configFilePath returns the path to ~/.config/mimir/config.json.
func configFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	// configDir returns the workspaces sub-dir; go one level up for the global config.
	return filepath.Join(filepath.Dir(dir), "config.json"), nil
}
