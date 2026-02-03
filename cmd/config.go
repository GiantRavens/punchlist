package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"punchlist/config"

	"github.com/spf13/cobra"
)

// create the config command
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Open the current configuration in your editor",
		Run: func(cmd *cobra.Command, args []string) {
			root, err := config.FindPunchlistRoot()
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				return
			}

			configPath := filepath.Join(root, config.PunchlistDir, "config.yaml")
			if _, err := os.Stat(configPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error locating config: %v\n", err)
				return
			}

			editorCommand, editorArgs := resolveEditorCommand()
			if shouldStartInsert(editorCommand) {
				editorArgs = append(editorArgs, "+startinsert")
			}
			if shouldStartGoyo(editorCommand) {
				editorArgs = append(editorArgs, goyoArgs()...)
			}
			editorArgs = append(editorArgs, configPath)

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
	cmd.AddCommand(newConfigMigrateCmd())
	return cmd
}

func newConfigMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Backfill missing config fields with defaults",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				if printNotPunchlistError(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				return
			}

			changed := false
			if len(cfg.States) == 0 {
				cfg.States = config.DefaultStates()
				changed = true
			}
			if len(cfg.LsStateOrder) == 0 {
				cfg.LsStateOrder = config.DefaultLsStateOrder()
				changed = true
			}

			if _, err := config.BuildStateCatalog(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Config invalid: %v\n", err)
				return
			}

			if !changed {
				fmt.Println("Config already up to date.")
				return
			}
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				return
			}
			fmt.Println("Config updated.")
		},
	}
}
