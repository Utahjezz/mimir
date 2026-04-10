package commands

import (
	"time"

	setupCmd "github.com/Utahjezz/mimir/internal/commands/setup"
	workspaceCmd "github.com/Utahjezz/mimir/internal/commands/workspace"
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
	searchCmd.Flags().IntVar(&searchLimit, "limit", 0, "Maximum number of results to return (0 = unlimited)")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Output results as JSON")
	searchCmd.Flags().BoolVar(&searchNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	searchCmd.Flags().StringVar(&searchWorkspace, "workspace", "", "Fan out search across all repos in this workspace")
	root.AddCommand(searchCmd)

	// report
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "Output report as JSON")
	reportCmd.Flags().BoolVar(&reportNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	root.AddCommand(reportCmd)

	// imports
	importsCmd.Flags().StringVar(&importsFile, "file", "", "Filter by source file path")
	importsCmd.Flags().StringVar(&importsModule, "module", "", "Filter by imported module/package path")
	importsCmd.Flags().BoolVar(&importsJSON, "json", false, "Output results as JSON")
	importsCmd.Flags().BoolVar(&importsNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	importsCmd.Flags().StringVar(&importsWorkspace, "workspace", "", "Fan out imports query across all repos in this workspace")
	root.AddCommand(importsCmd)

	// refs
	refsCmd.Flags().StringVar(&refsCaller, "caller", "", "Filter by caller symbol name")
	refsCmd.Flags().StringVar(&refsCallee, "callee", "", "Filter by callee name")
	refsCmd.Flags().StringVar(&refsFile, "file", "", "Filter by caller file path")
	refsCmd.Flags().BoolVar(&refsJSON, "json", false, "Output results as JSON")
	refsCmd.Flags().BoolVar(&refsHotspot, "hotspot", false, "Print the top-N most-called symbols ranked by inbound call count")
	refsCmd.Flags().IntVar(&refsLimit, "limit", 20, "Number of results to return for --hotspot (default 20)")
	refsCmd.Flags().BoolVar(&refsNoRefresh, "no-refresh", false, "Skip automatic re-index before querying")
	refsCmd.Flags().StringVar(&refsWorkspace, "workspace", "", "Fan out refs query across all repos in this workspace")
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

	// impact + impact simulate
	impactSimulateCmd.Flags().StringVar(&impactSimulateSymbol, "symbol", "", "Target symbol to simulate change against (required)")
	impactSimulateCmd.Flags().StringVar(&impactSimulateChangeRaw, "change", "", "Hypothetical change descriptor: kind[:key=value[:key=value...]]")
	impactSimulateCmd.Flags().IntVar(&impactSimulateMaxDepth, "max-depth", 6, "Maximum graph depth to analyze (0 = unlimited)")
	impactSimulateCmd.Flags().BoolVar(&impactSimulateCrossRepo, "cross-repo", true, "Include cross-repo workspace links in analysis")
	impactSimulateCmd.Flags().BoolVar(&impactSimulateJSON, "json", false, "Output full impact simulation result as JSON (impact-sim/v1)")
	impactSimulateCmd.Flags().BoolVar(&impactSimulateNoRefresh, "no-refresh", false, "Skip automatic re-index before simulation")
	impactSimulateCmd.Flags().StringVar(&impactSimulateWorkspace, "workspace", "", "Workspace name for cross-repo context (default: active workspace)")
	impactCmd.AddCommand(impactSimulateCmd)
	root.AddCommand(impactCmd)

	// workspace
	root.AddCommand(workspaceCmd.WorkspaceCmd)

	// setup
	root.AddCommand(setupCmd.SetupCmd)
}
