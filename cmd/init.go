package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"punchlist/config"

	"github.com/spf13/cobra"
)

// create the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new punchlist project",
		Run: func(cmd *cobra.Command, args []string) {
			// resolve working directory for project setup
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current directory: %v\n", err)
				return
			}

			punchlistPath := filepath.Join(cwd, config.PunchlistDir)
			// avoid overwriting an existing init
			if _, err := os.Stat(punchlistPath); !os.IsNotExist(err) {
				fmt.Println("Punchlist project already initialized.")
				return
			}

			// create the config directory
			if err := os.MkdirAll(punchlistPath, 0755); err != nil {
				fmt.Printf("Error creating .punchlist directory: %v\n", err)
				return
			}

			// write a default config
			defaultConfig := &config.Config{
				NextID:       1,
				IDWidth:      config.DefaultIDWidth(),
				LsStateOrder: config.DefaultLsStateOrder(),
			}
			if err := config.SaveConfig(defaultConfig); err != nil {
				fmt.Printf("Error creating default config: %v\n", err)
				return
			}

			fmt.Println("Punchlist project initialized successfully.")
		},
	}
	return cmd
}
