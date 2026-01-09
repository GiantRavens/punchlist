package cmd

import (
	"errors"
	"fmt"

	"punchlist/config"
)

const notPunchlistMessage = "No tasks found - this is not a .punchlist directory. To make it one, run pin init"

func printNotPunchlistError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, config.ErrPunchlistNotFound) {
		fmt.Println(notPunchlistMessage)
		return true
	}
	return false
}
