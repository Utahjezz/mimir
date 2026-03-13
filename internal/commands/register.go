package commands

import "github.com/spf13/cobra"

// Register adds all subcommands and their flags to root.
func Register(root *cobra.Command) {
	// index
	indexCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output stats as JSON")
	indexCmd.Flags().BoolVar(&indexRebuild, "rebuild", false, "Drop the existing index and reindex from scratch")
	root.AddCommand(indexCmd)

	// symbols + symbol (no flags)
	root.AddCommand(symbolsCmd)

	// symbol
	symbolCmd.Flags().StringVar(&symbolType, "type", "", "Narrow by symbol type (function, method, class, ...)")
	symbolCmd.Flags().BoolVar(&symbolJSON, "json", false, "Output results as JSON")
	root.AddCommand(symbolCmd)

	// search
	searchCmd.Flags().StringVar(&searchName, "name", "", "Exact symbol name match")
	searchCmd.Flags().StringVar(&searchLike, "like", "", "Symbol name prefix (LIKE)")
	searchCmd.Flags().StringVar(&searchFuzzy, "fuzzy", "", "FTS5 fuzzy name match (supports prefix 'Foo*', multi-token 'foo bar')")
	searchCmd.Flags().StringVar(&searchType, "type", "", "Symbol type (function, method, class, ...)")
	searchCmd.Flags().StringVar(&searchFile, "file", "", "Filter by file path")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Output results as JSON")
	root.AddCommand(searchCmd)

	// report
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "Output report as JSON")
	root.AddCommand(reportCmd)

	// refs
	refsCmd.Flags().StringVar(&refsCaller, "caller", "", "Filter by caller symbol name")
	refsCmd.Flags().StringVar(&refsCallee, "callee", "", "Filter by callee name")
	refsCmd.Flags().StringVar(&refsFile, "file", "", "Filter by caller file path")
	refsCmd.Flags().BoolVar(&refsJSON, "json", false, "Output results as JSON")
	root.AddCommand(refsCmd)

	// tree
	treeCmd.Flags().BoolVar(&treeJSON, "json", false, "Output tree as JSON")
	treeCmd.Flags().BoolVar(&treeFiles, "files", false, "Show individual files under each directory")
	treeCmd.Flags().IntVar(&treeDepth, "depth", 0, "Limit directory depth (0 = unlimited)")
	root.AddCommand(treeCmd)

	// callers
	callersCmd.Flags().BoolVar(&callersJSON, "json", false, "Output results as JSON")
	callersCmd.Flags().IntVar(&callersDepth, "depth", 2, "Recursion depth for caller traversal (0 = unlimited)")
	root.AddCommand(callersCmd)

	// dead
	deadCmd.Flags().StringVar(&deadType, "type", "", "Restrict to symbol type (function, method, ...)")
	deadCmd.Flags().StringVar(&deadFile, "file", "", "Filter by file path substring")
	deadCmd.Flags().BoolVar(&deadUnexported, "unexported", false, "Only show unexported symbols (reduces false positives)")
	deadCmd.Flags().BoolVar(&deadJSON, "json", false, "Output results as JSON")
	root.AddCommand(deadCmd)
}
