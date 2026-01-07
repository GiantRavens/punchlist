package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pin",
		Aliases: []string{"punchlist"},
		Short:   "A text-native, AI-friendly task and ticket system.",
		Long: `punchlist is a Markdown-first task and ticket system designed for people who think and work in text.
Each task is a single Markdown file with YAML frontmatter.
The CLI provides a concise, human-readable grammar for creating, updating, querying, and annotating tasks.`,
	}

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newDeferCmd())
	cmd.AddCommand(newBlockCmd())
	cmd.AddCommand(newConfirmCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newLogCmd())
	cmd.AddCommand(newDueCmd())
	cmd.AddCommand(newNoteCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newConfigCmd())

	return cmd
}

func Execute() {
	root := NewRootCmd()
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && !isSubcommand(root, args[0]) {
		if err := createTaskFromArgs(args); err != nil {
			fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
			os.Exit(1)
		}
		return
	}

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

func isSubcommand(root *cobra.Command, name string) bool {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name || cmd.HasAlias(name) {
			return true
		}
	}
	return false
}
