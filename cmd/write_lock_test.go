package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// findCommandByPath walks the command tree for a leaf whose CommandPath matches.
func findCommandByPath(root *cobra.Command, path string) *cobra.Command {
	if root.CommandPath() == path {
		return root
	}
	for _, c := range root.Commands() {
		if found := findCommandByPath(c, path); found != nil {
			return found
		}
	}
	return nil
}

// Every command that mutates a task file MUST be gated by the write lock.
// This locks the 1.3.6/1.3.7 mutators that were initially omitted from the
// mutatesPunchlist allowlist (they wrote unlocked — a real concurrency bug).
func TestMutatorsRequireWriteLock(t *testing.T) {
	root := NewRootCmd()
	mutators := []string{
		"pin section set", "pin section add", "pin problem", "pin approach",
		"pin acceptance add", "pin acceptance rm", "pin acceptance set",
		"pin note", "pin note rm", "pin check", "pin due", "pin pri",
		"pin tag", "pin tag set", "pin tag rm",
		"pin meta", "pin del", "pin compact",
	}
	for _, path := range mutators {
		cmd := findCommandByPath(root, path)
		if cmd == nil {
			t.Errorf("command %q not registered", path)
			continue
		}
		if !mutatesPunchlist(cmd) {
			t.Errorf("%q mutates task files but is NOT gated by the write lock", path)
		}
	}
}

// Read-only commands must NOT take the lock (so they never block on a writer).
func TestReadersDoNotRequireWriteLock(t *testing.T) {
	root := NewRootCmd()
	readers := []string{"pin ls", "pin show", "pin search", "pin acceptance", "pin note ls", "pin deps", "pin doctor"}
	for _, path := range readers {
		cmd := findCommandByPath(root, path)
		if cmd == nil {
			t.Errorf("command %q not registered", path)
			continue
		}
		if mutatesPunchlist(cmd) {
			t.Errorf("%q is read-only but is gated as a mutator", path)
		}
	}
}
