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

	workspaceListCmd.Flags().BoolVar(&workspaceListJSON, "json", false, "Output results as JSON")
	workspaceCmd.AddCommand(workspaceListCmd)

	workspaceDeleteCmd.Flags().BoolVar(&workspaceDeleteConfirm, "confirm", false, "Confirm permanent deletion of the workspace")
	workspaceCmd.AddCommand(workspaceDeleteCmd)

	workspaceLinkCmd.Flags().StringVar(&workspaceLinkSrcFile, "src-file", "", "Disambiguate src symbol by file path suffix")
	workspaceLinkCmd.Flags().StringVar(&workspaceLinkDstFile, "dst-file", "", "Disambiguate dst symbol by file path suffix")
	workspaceLinkCmd.Flags().StringVar(&workspaceLinkNote, "note", "", "Human-readable description of the link")
	workspaceLinkCmd.Flags().StringArrayVar(&workspaceLinkMeta, "meta", nil, "Metadata key=value pair (repeatable)")
	workspaceCmd.AddCommand(workspaceLinkCmd)

	workspaceLinksCmd.Flags().StringVar(&workspaceLinksFrom, "from", "", "Filter links by source repo path (defaults to current directory)")
	workspaceLinksCmd.Flags().BoolVar(&workspaceLinksJSON, "json", false, "Output results as JSON")
	workspaceLinksCmd.Flags().StringVar(&workspaceLinksSrcSymbol, "src-symbol", "", "Filter links by source symbol name (exact match)")
	workspaceLinksCmd.Flags().StringVar(&workspaceLinksDstSymbol, "dst-symbol", "", "Filter links by destination symbol name (exact match)")
	workspaceLinksCmd.Flags().BoolVar(&workspaceLinksCheck, "check", false, "Validate that symbols and file paths still exist in their respective repositories")
	workspaceCmd.AddCommand(workspaceLinksCmd)

	workspaceCmd.AddCommand(workspaceUnlinkCmd)
}
