package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"punchlist/config"
)

const notPunchlistMessage = "No tasks found - this is not a .punchlist directory. To make it one, run pin init"

// exitCode is the process exit status recorded by failf. Error paths inside
// cobra Run funcs must not os.Exit directly — PersistentPostRun releases the
// project write lock only after Run returns — so they record failure here and
// Execute terminates with it once post-run has completed.
var exitCode int

// failf reports a command error to stderr and marks the run as failed.
func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	exitCode = 1
}

// exitIfFailed terminates the process non-zero if any failf fired.
func exitIfFailed() {
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func printNotPunchlistError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, config.ErrPunchlistNotFound) {
		failf("%s\n", notPunchlistHint())
		return true
	}
	return false
}

// notPunchlistHint returns a context-aware version of notPunchlistMessage.
// If cwd looks like the tasks/ folder of an existing punchlist, point the
// user up one level instead of suggesting `pin init`.
func notPunchlistHint() string {
	cwd, err := os.Getwd()
	if err != nil || filepath.Base(cwd) != "tasks" {
		return notPunchlistMessage
	}
	parent := filepath.Dir(cwd)
	info, err := os.Stat(filepath.Join(parent, config.PunchlistDir))
	if err != nil || !info.IsDir() {
		return notPunchlistMessage
	}
	return fmt.Sprintf("This looks like the tasks/ folder of a punchlist at %s. Run pin one directory up (cd ..).", parent)
}
