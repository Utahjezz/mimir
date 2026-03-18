package commands

import (
	"time"

	"github.com/spf13/cobra"
)

// RefreshThreshold is the minimum age of the index before a query command
// triggers an automatic re-walk. Commands read this value when calling
// AutoRefresh. It is set by the --refresh-threshold root flag (default 10s).
var RefreshThreshold = 10 * time.Second

// Register adds all subcommands and their flags to root.
func Register(root *cobra.Command) {
	// Global flags (available to all subcommands).
	root.PersistentFlags().DurationVar(&RefreshThreshold, "refresh-threshold", 10*time.Second,
		"Minimum index age before a query triggers an automatic re-index (e.g. 10s, 2m, 0s)")

	// index
	indexCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output stats as JSON")
	indexCmd.Flags().BoolVar(&indexRebuild, "rebuild", false, "Drop the existing index and reindex from scratch")
	root.AddCommand(indexCmd)

	// symbols + symbol (no flags)
	root.AddCommand(symbolsCmd)

	// symbol
	symbolCmd.Flags().StringVar(&symbolType, "type", "", "Narrow by symbol type (function, method, class, ...)")
	symbolCmd.Flags().BoolVar(&symbolJSON, "json", false, "Output results as JSON")
	symbolCmd.Flags().BoolVar(&symbolNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(symbolCmd)

	// search
	searchCmd.Flags().StringVar(&searchName, "name", "", "Exact symbol name match")
	searchCmd.Flags().StringVar(&searchLike, "like", "", "Symbol name prefix (LIKE)")
	searchCmd.Flags().StringVar(&searchFuzzy, "fuzzy", "", "FTS5 fuzzy name match (supports prefix 'Foo*', multi-token 'foo bar')")
	searchCmd.Flags().StringVar(&searchType, "type", "", "Symbol type (function, method, class, ...)")
	searchCmd.Flags().StringVar(&searchFile, "file", "", "Filter by file path")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Output results as JSON")
	searchCmd.Flags().BoolVar(&searchNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(searchCmd)

	// report
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "Output report as JSON")
	reportCmd.Flags().BoolVar(&reportNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(reportCmd)

	// refs
	refsCmd.Flags().StringVar(&refsCaller, "caller", "", "Filter by caller symbol name")
	refsCmd.Flags().StringVar(&refsCallee, "callee", "", "Filter by callee name")
	refsCmd.Flags().StringVar(&refsFile, "file", "", "Filter by caller file path")
	refsCmd.Flags().BoolVar(&refsJSON, "json", false, "Output results as JSON")
	refsCmd.Flags().BoolVar(&refsHotspot, "hotspot", false, "Print the top-N most-called symbols ranked by inbound call count")
	refsCmd.Flags().IntVar(&refsLimit, "limit", 20, "Number of results to return for --hotspot (default 20)")
	refsCmd.Flags().BoolVar(&refsNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(refsCmd)

	// tree
	treeCmd.Flags().BoolVar(&treeJSON, "json", false, "Output tree as JSON")
	treeCmd.Flags().BoolVar(&treeFiles, "files", false, "Show individual files under each directory")
	treeCmd.Flags().IntVar(&treeDepth, "depth", 0, "Limit directory depth (0 = unlimited)")
	treeCmd.Flags().BoolVar(&treeNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(treeCmd)

	// callers
	callersCmd.Flags().BoolVar(&callersJSON, "json", false, "Output results as JSON")
	callersCmd.Flags().IntVar(&callersDepth, "depth", 2, "Recursion depth for caller traversal (0 = unlimited)")
	callersCmd.Flags().BoolVar(&callersNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(callersCmd)

	// dead
	deadCmd.Flags().StringVar(&deadType, "type", "", "Restrict to symbol type (function, method, ...)")
	deadCmd.Flags().StringVar(&deadFile, "file", "", "Filter by file path substring")
	deadCmd.Flags().BoolVar(&deadUnexported, "unexported", false, "Only show unexported symbols (reduces false positives)")
	deadCmd.Flags().BoolVar(&deadJSON, "json", false, "Output results as JSON")
	deadCmd.Flags().BoolVar(&deadNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(deadCmd)
}
