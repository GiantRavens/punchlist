package projectlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	lockDirName = "write.lock"
	ownerName   = "owner.json"
)

type owner struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// Lock is a project-scoped mutation lease. Acquisition is an atomic mkdir,
// which works on both local disks and the synced filesystems Punchlist uses.
type Lock struct {
	path string
}

// Acquire waits briefly for the scope's writer and safely reaps abandoned
// leases. root must be the directory containing .punchlist.
func Acquire(root string, timeout time.Duration) (*Lock, error) {
	path := filepath.Join(root, ".punchlist", lockDirName)
	deadline := time.Now().Add(timeout)
	for {
		err := os.Mkdir(path, 0700)
		if err == nil {
			l := &Lock{path: path}
			if err := l.writeOwner(); err != nil {
				_ = os.RemoveAll(path)
				return nil, err
			}
			return l, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create punchlist write lock: %w", err)
		}
		if reapable(path) {
			stale := path + ".stale-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
			if os.Rename(path, stale) == nil {
				_ = os.RemoveAll(stale)
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("punchlist is busy: timed out waiting for %s", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (l *Lock) writeOwner() error {
	b, err := json.Marshal(owner{PID: os.Getpid(), StartedAt: time.Now().UTC()})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(l.path, ownerName), append(b, '\n'), 0600); err != nil {
		return fmt.Errorf("write punchlist lock owner: %w", err)
	}
	return nil
}

func reapable(path string) bool {
	info, statErr := os.Stat(path)
	if statErr == nil && time.Since(info.ModTime()) > 10*time.Minute {
		return true // PID may have been reused; Punchlist mutations are bounded and short
	}
	b, err := os.ReadFile(filepath.Join(path, ownerName))
	if err == nil {
		var o owner
		if json.Unmarshal(b, &o) == nil && o.PID > 0 && processAlive(o.PID) {
			return false
		}
	}
	return statErr == nil && time.Since(info.ModTime()) > 30*time.Second
}

// Release removes only this process's lease. A renamed/reaped lease is left
// alone rather than risking removal of a successor writer's directory.
func (l *Lock) Release() error {
	b, err := os.ReadFile(filepath.Join(l.path, ownerName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var o owner
	if json.Unmarshal(b, &o) != nil || o.PID != os.Getpid() {
		return nil
	}
	return os.RemoveAll(l.path)
}
