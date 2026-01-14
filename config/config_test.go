package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func sandboxRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("Failed to determine test file path")
	}
	packageDir := filepath.Dir(file)
	projectRoot := filepath.Dir(packageDir)
	return filepath.Join(projectRoot, "sandbox")
}

// test punchlist dir discovery
func TestFindPunchlistDir(t *testing.T) {
	sandboxDir := sandboxRoot(t)
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox dir: %v", err)
	}
	defer os.RemoveAll(sandboxDir)

	// test case 1: .punchlist in current directory
	t.Run("finds .punchlist in current dir", func(t *testing.T) {
		testDir := filepath.Join(sandboxDir, "test1")
		punchlistDir := filepath.Join(testDir, PunchlistDir)
		if err := os.MkdirAll(punchlistDir, 0755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		foundDir, err := findPunchlistDir(testDir)
		if err != nil {
			t.Errorf("Expected to find .punchlist dir, but got error: %v", err)
		}
		if foundDir != punchlistDir {
			t.Errorf("Expected dir %s, but got %s", punchlistDir, foundDir)
		}
	})

	// test case 3: no .punchlist directory
	t.Run("returns error when no .punchlist dir", func(t *testing.T) {
		tempRoot := t.TempDir()
		testDir := filepath.Join(tempRoot, "test3")
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		_, err := findPunchlistDir(testDir)
		if err == nil {
			t.Errorf("Expected an error, but got none")
		}
	})
}

// test load and save config round-trip
func TestLoadAndSaveConfig(t *testing.T) {
	sandboxDir := sandboxRoot(t)
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox dir: %v", err)
	}
	defer os.RemoveAll(sandboxDir)

	testDir := filepath.Join(sandboxDir, "test_load_save")
	punchlistDir := filepath.Join(testDir, PunchlistDir)
	if err := os.MkdirAll(punchlistDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	// change to the test directory to test relative paths
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("Failed to change directory to %s: %v", testDir, err)
	}
	defer os.Chdir(originalWd)
	t.Run("loads a saved config", func(t *testing.T) {
		cfg := &Config{NextID: 42}
		if err := SaveConfig(cfg); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		loadedCfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if loadedCfg.NextID != cfg.NextID {
			t.Errorf("Expected NextID to be %d, but got %d", cfg.NextID, loadedCfg.NextID)
		}
	})

	t.Run("load returns error if config does not exist", func(t *testing.T) {
		// make sure config file is not there
		os.Remove(filepath.Join(punchlistDir, "config.yaml"))
		_, err := LoadConfig()
		if err == nil {
			t.Errorf("Expected an error when loading non-existent config, but got none")
		}
	})
}
