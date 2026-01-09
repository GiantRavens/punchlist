package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"punchlist/config"
	"punchlist/task"
)

func punchlistRoot() (string, error) {
	return config.FindPunchlistRoot()
}

func tasksDir() (string, error) {
	root, err := punchlistRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "tasks"), nil
}

func trashDir() (string, error) {
	root, err := punchlistRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".trash"), nil
}

func isPathToken(token string) bool {
	return strings.HasPrefix(token, ".") || strings.HasPrefix(token, "/")
}

func extractTargetPath(args []string) (string, []string) {
	if len(args) == 0 {
		return "", args
	}
	if isPathToken(args[0]) {
		return args[0], args[1:]
	}
	if _, ok := task.ParseState(args[0]); ok && len(args) > 1 && isPathToken(args[1]) {
		remaining := append([]string{args[0]}, args[2:]...)
		return args[1], remaining
	}
	return "", args
}

func punchlistRootFromPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %w", err)
	}
	info, err := os.Stat(filepath.Join(absPath, config.PunchlistDir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", config.ErrPunchlistNotFound
		}
		return "", fmt.Errorf("could not access %s: %w", config.PunchlistDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s exists but is not a directory at %s", config.PunchlistDir, absPath)
	}
	return absPath, nil
}

func withWorkingDir(dir string, fn func() error) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer os.Chdir(cwd)
	return fn()
}
