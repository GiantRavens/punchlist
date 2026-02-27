package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"punchlist/task"
)

// build the root command and all subcommands
func NewRootCmd() *cobra.Command {
	// explain the tool up front in help output
	longDesc := `Punchlist is a markdown-first task ticket system. Each task is a single markdown file
with yaml frontmatter, kept in plain folders. The result is non-proprietary,
easy to parse, and aligned with the markdown philosophy. It works great with
Obsidian and any text-first workflow.

Conversational grammar for tasks:
  pin STATE "task title" [pri:n] [by:date] [tags:{a,b}]

State and modifiers are optional. If you omit state, it defaults to TODO.
Priority and dates are always optional.

Examples:
  pin "write outline"
  pin todo "draft messaging brief" pri:1
  pin done "shipped quick fix for onboarding typo"
  pin todo "send pr draft" by:2026-01-15
  pin todo "ship notes" by:tomorrow
  pin todo "review plan" by:friday
  pin todo ../work "queue follow-up"
  pin begun "triage the backlog"
  pin block "waiting on vendor response"

List and modify tasks:
  pin ls
  pin ls ../work
  pin ls todo --tag launch
  pin search exagrid
  pin due 12 "next tuesday"
  pin note 12 "ask for feedback from legal"
  pin tag 12 15 "today, blocked"
  pin edit 12
  pin del 12
  pin compact

States, aliases, and browse hotkeys are configured in .punchlist/config.yaml (single-word tokens).
Config highlights: edit_goyo enables +Goyo for vim/nvim; browse_margin widens browse gutters.

Zsh cwd hook snippet (optional, for prompt or env):
  autoload -U add-zsh-hook
  _pin_set_root() {
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
      if [[ -d "$dir/.punchlist" ]]; then
        export PUNCHLIST_ROOT="$dir"
        return
      fi
      dir="${dir:h}"
    done
    unset PUNCHLIST_ROOT
  }
  add-zsh-hook chpwd _pin_set_root
  _pin_set_root`

	cmd := &cobra.Command{
		Use:               "pin",
		Aliases:           []string{"punchlist"},
		Short:             "A text-native, AI-friendly task and ticket system.",
		Long:              longDesc,
		ValidArgsFunction: rootArgCompletion,
		Version:           Version,
	}
	cmd.SetVersionTemplate("pin {{.Version}}\n")

	cmd.AddCommand(newBlockCmd())
	cmd.AddCommand(newBrowseCmd())
	cmd.AddCommand(newCompactCmd())
	cmd.AddCommand(newConfirmCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newDeferCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newDueCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newNoteCmd())
	cmd.AddCommand(newSearchCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newTodoCmd())
	cmd.AddCommand(newTagCmd())
	cmd.AddCommand(newMetaCmd())
	cmd.AddCommand(newAcceptanceCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newDepsCmd())

	// keep completion available but hidden from help
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.InitDefaultCompletionCmd()

	return cmd
}

// execute the cli, supporting implicit task creation
func Execute() {
	// build the root command tree
	root := NewRootCmd()
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") && !isSubcommand(root, args[0]) && !isCobraCompletionCmd(args[0]) {
		if len(args) > 1 {
			if ids, err := parseTaskIDs(args[1:]); err == nil {
				stateCatalog, err := loadStateCatalog()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading state config: %v\n", err)
					os.Exit(1)
				}
				newState := task.State(stateCatalog.Canonicalize(args[0]))
				updateTaskState(ids, newState)
				return
			}
		}
		// treat bare args as task creation
		if err := createTaskFromArgs(args); err != nil {
			if printNotPunchlistError(err) {
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
			os.Exit(1)
		}
		return
	}

	// run cobra command execution
	if err := root.Execute(); err != nil {
		if printNotPunchlistError(err) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

// check if a token matches a subcommand name or alias
func isSubcommand(root *cobra.Command, name string) bool {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name || cmd.HasAlias(name) {
			return true
		}
	}
	return false
}

// ignore cobra completion shim commands during implicit creation
func isCobraCompletionCmd(name string) bool {
	return name == "__complete" || name == "__completeNoDesc"
}
