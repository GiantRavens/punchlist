package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"punchlist/config"
)

const notPunchlistMessage = "No tasks found - this is not a .punchlist directory. To make it one, run pin init"

func printNotPunchlistError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, config.ErrPunchlistNotFound) {
		fmt.Fprintln(os.Stderr, notPunchlistHint())
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
