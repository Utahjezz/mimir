package commands

import (
	"fmt"

	"github.com/Utahjezz/mimir/pkg/graph"
	"github.com/Utahjezz/mimir/pkg/indexer"
	workspacepkg "github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	graphFormat    string
	graphJSON      bool
	graphScope     string
	graphNoRefresh bool

	// Active filter flags (wired in subtask_06).
	graphFocus    string
	graphExclude  string
	graphMinCalls int
	graphDepth    int

	// Workspace name for multi-repo graph enrichment.
	graphWorkspace string
)

var graphCmd = &cobra.Command{
	Use:   "graph <root>",
	Short: "Generate architectural dependency graph from indexed code",
	Long: `Build and render a dependency graph from the call refs stored in the index.
The graph is aggregated at the scope level (--scope), with default scope "package".
Output can be rendered as a Mermaid flowchart (default) or as structured JSON.

Use --json as a shorthand for --format json.

Filtering options (--focus, --exclude, --min-calls, --depth) narrow the graph
to relevant subgraphs. Use --workspace to enrich the graph with cross-repo
edges from workspace link declarations.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGraph,
}

func runGraph(cmd *cobra.Command, args []string) error {
	// Resolve output format: --json overrides --format.
	if graphJSON {
		graphFormat = "json"
	}
	if graphFormat != "mermaid" && graphFormat != "json" {
		return fmt.Errorf("unsupported --format %q; expected 'mermaid' or 'json'", graphFormat)
	}

	if len(args) == 0 {
		return fmt.Errorf("requires a [root] argument")
	}
	root := args[0]

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	if !graphNoRefresh {
		if _, err := indexer.AutoRefresh(root, db, RefreshThreshold); err != nil {
			return fmt.Errorf("auto-refresh: %w", err)
		}
	}

	refs, err := indexer.SearchRefs(db, indexer.RefQuery{})
	if err != nil {
		return fmt.Errorf("refs query failed: %w", err)
	}

	opts := graph.BuildOptions{
		Root:    root,
		IndexDB: db,
		Refs:    refs,
		Options: graph.GraphOptions{
			Scope:     graph.Scope(graphScope),
			Format:    graphFormat,
			Focus:     graphFocus,
			Exclude:   graphExclude,
			MinCalls:  graphMinCalls,
			Depth:     graphDepth,
			Workspace: graphWorkspace,
			NoRefresh: graphNoRefresh,
		},
	}

	g, err := graph.BuildGraph(opts)
	if err != nil {
		return fmt.Errorf("build graph: %w", err)
	}

	g = graph.FilterGraph(g, opts.Options)

	// Enrich with workspace links when --workspace is set.
	if opts.Options.Workspace != "" {
		wsDB, err := workspacepkg.OpenWorkspace(opts.Options.Workspace)
		if err == nil {
			defer wsDB.Close()
			if err := graph.EnrichWithWorkspaceLinks(g, wsDB, root); err != nil {
				return fmt.Errorf("workspace enrichment: %w", err)
			}
		}
		// Silently skip when workspace doesn't exist or open fails.
	}

	switch graphFormat {
	case "json":
		if err := graph.RenderJSON(g, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("render json: %w", err)
		}
	default:
		if err := graph.RenderMermaid(g, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("render mermaid: %w", err)
		}
	}

	return nil
}
