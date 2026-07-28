package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"punchlist/config"
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
	cmd.PersistentPreRunE = acquireCommandWriteLock
	cmd.PersistentPostRun = releaseCommandWriteLock

	cmd.AddCommand(newBlockCmd())
	cmd.AddCommand(newBrowseCmd())
	cmd.AddCommand(newCompactCmd())
	cmd.AddCommand(newConfirmCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newDeferCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newDueCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newNoteCmd())
	cmd.AddCommand(newPriCmd())
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
				// pin#42: `pin <word> <ids>` is a state change ONLY when <word> is a KNOWN
				// state token. An unknown word (e.g. `pin get 486`) must NOT be minted into
				// an arbitrary state and applied — that silently moved real tasks into a bogus
				// "GET" state. Refuse loudly and suggest the likely intent instead.
				canonical, known := stateCatalog.Resolve(args[0])
				if !known {
					fmt.Fprint(os.Stderr, unknownStateMessage(args[0], args[1:], stateCatalog))
					os.Exit(1)
				}
				newState := task.State(canonical)
				rootDir, err := config.FindPunchlistRoot()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error resolving punchlist scope: %v\n", err)
					os.Exit(1)
				}
				if err := withProjectWriteLock(rootDir, func() error {
					updateTaskState(ids, newState)
					return nil
				}); err != nil {
					fmt.Fprintf(os.Stderr, "Error updating tasks: %v\n", err)
					os.Exit(1)
				}
				exitIfFailed()
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
		exitIfFailed()
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
	// error paths inside Run funcs record failure via failf instead of
	// exiting mid-command (the write lock releases in PersistentPostRun);
	// honor that recorded status now that post-run has completed.
	exitIfFailed()
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

// unknownStateMessage builds the pin#42 refusal shown when `pin <word> <ids>` names
// a word that is NOT a known state token. It never mutates tasks — it explains why
// and points at the likely intent (a read verb like `get`/`show` almost always meant
// `pin show`), then lists the valid state tokens.
func unknownStateMessage(token string, idArgs []string, catalog *config.StateCatalog) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Refusing: %q is not a known state, so it will not be applied to task(s) %s.\n",
		token, strings.Join(idArgs, " "))
	// A read-ish verb almost certainly meant `pin show`.
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "get", "show", "view", "see", "info", "read", "open", "cat", "inspect":
		fmt.Fprintf(&b, "  Did you mean:  pin show %s\n", strings.Join(idArgs, " "))
	}
	if states := stateTokenList(catalog); states != "" {
		fmt.Fprintf(&b, "  Valid states:  %s\n", states)
	}
	fmt.Fprintf(&b, "  To create a task with this title instead:  pin todo %q\n",
		strings.Join(append([]string{token}, idArgs...), " "))
	return b.String()
}

// stateTokenList returns the canonical state names (lowercased) in display order,
// for the unknown-state error's "Valid states" line.
func stateTokenList(catalog *config.StateCatalog) string {
	if catalog == nil {
		return ""
	}
	parts := []string{}
	for _, st := range catalog.SortStates() {
		parts = append(parts, strings.ToLower(st.Name))
	}
	return strings.Join(parts, ", ")
}
