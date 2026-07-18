package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// punchlist config directory name
const PunchlistDir = ".punchlist"

// ErrPunchlistNotFound indicates no punchlist directory exists in scope.
var ErrPunchlistNotFound = errors.New("punchlist directory not found")

// config holds persisted settings for a punchlist scope
type Config struct {
	NextID          int           `yaml:"next_id"`
	IDWidth         int           `yaml:"id_width,omitempty"`
	LsStateOrder    []string      `yaml:"ls_state_order,omitempty"`
	States          []StateConfig `yaml:"states,omitempty"`
	EditStartInsert *bool         `yaml:"edit_start_insert,omitempty"`
	EditGoyo        *bool         `yaml:"edit_goyo,omitempty"`
	BrowseMargin    int           `yaml:"browse_margin,omitempty"`
	TitleMaxLen     int           `yaml:"title_max_len,omitempty"`
	LsTitleMaxLen   int           `yaml:"ls_title_max_len,omitempty"`
	Accepting       *bool         `yaml:"accepting,omitempty"`
}

// whether this scope accepts new tasks (default true; set accepting: false
// in config.yaml to close a scope to new tasks while ls/done/note keep working)
func (c *Config) IsAccepting() bool {
	return c == nil || c.Accepting == nil || *c.Accepting
}

// default id width for filename padding
func DefaultIDWidth() int {
	return 3
}

// default state order for ls
func DefaultLsStateOrder() []string {
	return []string{"TODO", "BEGUN", "BLOCKED", "FOLLOWUP", "DEFER", "NOTDO", "DONE"}
}

// default editor insert-mode behavior for vim-style editors
func DefaultEditStartInsert() bool {
	return true
}

// default goyo invocation for vim/nvim
func DefaultEditGoyo() bool {
	return false
}

// default browse margin (columns per side)
func DefaultBrowseMargin() int {
	return 12
}

// default max length for stored task titles (also influences filename slug)
func DefaultTitleMaxLen() int {
	return 80
}

// default max length for titles shown in pin ls
func DefaultLsTitleMaxLen() int {
	return 80
}

// find the punchlist directory in the current working directory
func findPunchlistDir(startDir string) (string, error) {
	punchlistPath := filepath.Join(startDir, PunchlistDir)
	info, err := os.Stat(punchlistPath)
	if err == nil && info.IsDir() {
		return punchlistPath, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("could not access %s directory: %w", PunchlistDir, err)
	}
	return "", ErrPunchlistNotFound
}

func FindPunchlistRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get current working directory: %w", err)
	}

	punchlistPath, err := findPunchlistDir(cwd)
	if err != nil {
		return "", err
	}

	return filepath.Dir(punchlistPath), nil
}

// resolve the punchlist root that should receive a new task, starting at
// startDir. The scope at startDir itself must exist (creation never walks up
// on its own); only a scope marked accepting: false is skipped, with the walk
// continuing up the directory tree until an accepting scope is found.
// Returns the accepting root and the closed roots skipped along the way.
func FindAcceptingRootFrom(startDir string) (string, []string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", nil, fmt.Errorf("could not resolve path: %w", err)
	}
	startAbs := dir

	var skipped []string
	for {
		punchlistPath, findErr := findPunchlistDir(dir)
		if findErr == nil {
			cfg, cfgErr := loadConfigAt(punchlistPath)
			if cfgErr != nil {
				return "", skipped, cfgErr
			}
			if cfg.IsAccepting() {
				return dir, skipped, nil
			}
			skipped = append(skipped, dir)
		} else if dir == startAbs {
			// the nearest scope must exist at the starting directory itself
			return "", nil, findErr
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", skipped, fmt.Errorf("punchlist scope at %s is closed to new tasks (accepting: false) and no accepting parent scope was found", skipped[0])
		}
		dir = parent
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

	return loadConfigAt(punchlistPath)
}

// load config from an explicit punchlist directory
func loadConfigAt(punchlistPath string) (*Config, error) {
	configPath := filepath.Join(punchlistPath, "config.yaml")
	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %w. please run 'pin init'", err)
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

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("could not write config temp file: %w", err)
	}
	return os.Rename(tmpPath, configPath)
}
