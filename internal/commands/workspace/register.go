package commands

import "github.com/spf13/cobra"

func Register(workspaceCmd *cobra.Command) {
	workspaceCmd.AddCommand(workspaceCreateCmd)

	workspaceCmd.AddCommand(workspaceAddCmd)

	workspaceCmd.AddCommand(workspaceUseCmd)

	workspaceCmd.AddCommand(workspaceRemoveCmd)

	workspaceShowCmd.Flags().BoolVar(&workspaceShowJSON, "json", false, "Output results as JSON")
	workspaceCmd.AddCommand(workspaceShowCmd)
}
