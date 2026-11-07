package cmd

import (
	"fmt"
	"os"
	"time"

	"punchlist/config"
	"punchlist/internal/projectlock"

	"github.com/spf13/cobra"
)

var commandWriteLock *projectlock.Lock

func acquireCommandWriteLock(cmd *cobra.Command, args []string) error {
	if !mutatesPunchlist(cmd) || commandWriteLock != nil {
		return nil
	}
	// help mutates nothing; taking the lock for it lets a reader that closes
	// the pipe early (pin todo --help | head) kill pin while it holds the lock
	if wantsHelp(args) {
		return nil
	}
	// read --body-file / --file sources BEFORE the lock: a stdin body that is
	// slow to arrive (or typed at a terminal) must not hold the scope (pin #21)
	if err := preloadTextSources(cmd, args); err != nil {
		return err
	}
	// Cross-scope task creation resolves and locks its destination itself.
	for _, arg := range args {
		if isPathToken(arg) {
			return nil
		}
	}
	root, err := config.FindPunchlistRoot()
	if err != nil {
		return err
	}
	commandWriteLock, err = projectlock.Acquire(root, 3*time.Second)
	return err
}

func releaseCommandWriteLock(_ *cobra.Command, _ []string) {
	if commandWriteLock == nil {
		return
	}
	if err := commandWriteLock.Release(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not release punchlist write lock: %v\n", err)
	}
	commandWriteLock = nil
}

func mutatesPunchlist(cmd *cobra.Command) bool {
	switch cmd.CommandPath() {
	case "pin todo", "pin start", "pin done", "pin notdo", "pin block", "pin confirm",
		"pin due", "pin note", "pin note rm", "pin pri",
		"pin tag", "pin tag set", "pin tag rm", "pin meta", "pin check",
		"pin acceptance add", "pin acceptance rm", "pin acceptance set",
		"pin section set", "pin section add", "pin problem", "pin approach",
		"pin del", "pin compact", "pin renumber":
		return true
	default:
		return false
	}
}

func withProjectWriteLock(root string, fn func() error) error {
	if commandWriteLock != nil {
		return fn()
	}
	l, err := projectlock.Acquire(root, 3*time.Second)
	if err != nil {
		return err
	}
	defer l.Release()
	return fn()
}
