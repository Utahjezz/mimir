package commands

import "github.com/spf13/cobra"

func Register(workspaceCmd *cobra.Command) {
	workspaceCmd.AddCommand(workspaceCreateCmd)

	workspaceCmd.AddCommand(workspaceAddCmd)

	workspaceCmd.AddCommand(workspaceUseCmd)

	workspaceCmd.AddCommand(workspaceRemoveCmd)

	workspaceIndexCmd.Flags().BoolVar(&workspaceIndexJSON, "json", false, "Output results as JSON")
	workspaceIndexCmd.Flags().BoolVar(&workspaceIndexRebuild, "rebuild", false, "Drop and recreate each index from scratch")
	workspaceIndexCmd.Flags().IntVar(&workspaceIndexConcurrency, "concurrency", defaultConcurrency, "Number of repositories to index concurrently")
	workspaceCmd.AddCommand(workspaceIndexCmd)

	workspaceShowCmd.Flags().BoolVar(&workspaceShowJSON, "json", false, "Output results as JSON")
	workspaceCmd.AddCommand(workspaceShowCmd)
}
