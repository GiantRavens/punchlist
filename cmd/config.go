package cmd

import (
	"fmt"
	"punchlist/config"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the current configuration",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}
			fmt.Printf("Next ID: %d\n", cfg.NextID)
		},
	}
	return cmd
}

