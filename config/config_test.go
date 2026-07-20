package config

import (
	"os"
	"path/filepath"
	"testing"
)

// test punchlist dir discovery
func TestFindPunchlistDir(t *testing.T) {
	// test case 1: .punchlist in current directory
	t.Run("finds .punchlist in current dir", func(t *testing.T) {
		testDir := t.TempDir()
		punchlistDir := filepath.Join(testDir, PunchlistDir)
		if err := os.MkdirAll(punchlistDir, 0755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		foundDir, err := findPunchlistDir(testDir)
		if err != nil {
			t.Errorf("Expected to find .punchlist dir, but got error: %v", err)
		}
		// TempDir itself may sit behind symlinks (macOS /var -> /private/var)
		want, _ := filepath.EvalSymlinks(punchlistDir)
		if foundDir != want {
			t.Errorf("Expected dir %s, but got %s", want, foundDir)
		}
	})

	// symlinked store: an engine-side convenience link must resolve to the
	// real store, or WalkDir lists zero tasks and mkdir forks a parallel store
	t.Run("resolves symlinked .punchlist to the real store", func(t *testing.T) {
		root := t.TempDir()
		realScope := filepath.Join(root, "state", "myforge")
		realStore := filepath.Join(realScope, PunchlistDir)
		if err := os.MkdirAll(realStore, 0755); err != nil {
			t.Fatalf("Failed to create real store: %v", err)
		}
		aliasScope := filepath.Join(root, "code", "myforge")
		if err := os.MkdirAll(aliasScope, 0755); err != nil {
			t.Fatalf("Failed to create alias scope: %v", err)
		}
		if err := os.Symlink(realStore, filepath.Join(aliasScope, PunchlistDir)); err != nil {
			t.Fatalf("Failed to create symlink: %v", err)
		}

		foundDir, err := findPunchlistDir(aliasScope)
		if err != nil {
			t.Fatalf("Expected to find symlinked .punchlist, but got error: %v", err)
		}
		want, _ := filepath.EvalSymlinks(realStore)
		if foundDir != want {
			t.Errorf("Expected resolved store %s, but got %s", want, foundDir)
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
	testDir := t.TempDir()
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
