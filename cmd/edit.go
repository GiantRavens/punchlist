package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"punchlist/config"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// create the edit command
func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit [id]",
		Short: "Open a task in your editor",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid task ID: %v\n", err)
				return
			}

			taskPath, err := findTaskFile(id)
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error finding task: %v\n", err)
				return
			}

			editorCommand, editorArgs := resolveEditorCommand()
			if shouldStartInsert(editorCommand) {
				editorArgs = append(editorArgs, "+startinsert")
			}
			if shouldStartGoyo(editorCommand) {
				editorArgs = append(editorArgs, goyoArgs()...)
			}
			editorArgs = append(editorArgs, taskPath)

			cmdExec := exec.Command(editorCommand, editorArgs...)
			cmdExec.Stdin = os.Stdin
			cmdExec.Stdout = os.Stdout
			cmdExec.Stderr = os.Stderr
			if err := cmdExec.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error launching editor: %v\n", err)
				return
			}
		},
	}
}

func resolveEditorCommand() (string, []string) {
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "nvim"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return "nvim", nil
	}
	return parts[0], parts[1:]
}

func shouldStartInsert(editorCommand string) bool {
	cfg, err := config.LoadConfig()
	startInsert := config.DefaultEditStartInsert()
	if err == nil && cfg.EditStartInsert != nil {
		startInsert = *cfg.EditStartInsert
	}
	if !startInsert {
		return false
	}
	editorName := strings.ToLower(filepath.Base(editorCommand))
	return editorName == "nvim" || editorName == "vim"
}

func shouldStartGoyo(editorCommand string) bool {
	cfg, err := config.LoadConfig()
	startGoyo := config.DefaultEditGoyo()
	if err == nil && cfg.EditGoyo != nil {
		startGoyo = *cfg.EditGoyo
	}
	if !startGoyo {
		return false
	}
	editorName := strings.ToLower(filepath.Base(editorCommand))
	return editorName == "nvim" || editorName == "vim"
}

func goyoArgs() []string {
	return []string{"+let g:goyo_quit_on_close=1", "+Goyo"}
}
