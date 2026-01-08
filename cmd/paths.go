package cmd

import (
	"path/filepath"
	"punchlist/config"
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
