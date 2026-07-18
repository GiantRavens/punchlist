package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

// newVersionCmd registers `pin version` as a real subcommand. Without it the
// bare word fell through to task creation — `pin version` created a task
// titled "version" (this scope's tasks 5, 14, and briefly 29/30).
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the pin version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("pin " + Version)
		},
	}
}
