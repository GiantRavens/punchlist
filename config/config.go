package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// punchlist config directory name
const PunchlistDir = ".punchlist"

// config holds persisted settings for a punchlist scope
type Config struct {
	NextID       int      `yaml:"next_id"`
	IDWidth      int      `yaml:"id_width,omitempty"`
	LsStateOrder []string `yaml:"ls_state_order,omitempty"`
}

// default id width for filename padding
func DefaultIDWidth() int {
	return 3
}

// default state order for ls
func DefaultLsStateOrder() []string {
	return []string{"BEGUN", "BLOCK", "TODO", "CONFIRM", "DONE", "NOTDO"}
}

// find the nearest punchlist directory by walking parents
func findPunchlistDir(startDir string) (string, error) {
	currentDir := startDir
	for {
		punchlistPath := filepath.Join(currentDir, PunchlistDir)
		info, err := os.Stat(punchlistPath)
		if err == nil && info.IsDir() {
			return punchlistPath, nil
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			return "", fmt.Errorf("could not find a %s directory in the current directory or any of its parents. please run 'punchlist init'", PunchlistDir)
		}
		currentDir = parent
	}
}

// load config from the nearest punchlist directory
func LoadConfig() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get current working directory: %w", err)
	}

	punchlistPath, err := findPunchlistDir(cwd)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(punchlistPath, "config.yaml")
	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %w. please run 'punchlist init'", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("could not decode config file: %w", err)
	}

	return &cfg, nil
}

// save config to the nearest punchlist directory
func SaveConfig(cfg *Config) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// we assume .punchlist exists when saving.
	punchlistPath, err := findPunchlistDir(cwd)
	if err != nil {
		return err
	}

	configPath := filepath.Join(punchlistPath, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}
