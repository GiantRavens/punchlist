package projectlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcquireSerializesWriters(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".punchlist"), 0755); err != nil {
		t.Fatal(err)
	}
	first, err := Acquire(root, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	_, err = Acquire(root, 40*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("expected busy timeout, got %v", err)
	}

	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(root, time.Second)
	if err != nil {
		t.Fatalf("lock was not reusable: %v", err)
	}
	defer second.Release()
}
