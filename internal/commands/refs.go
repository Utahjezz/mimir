package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	refsCaller    string
	refsCallee    string
	refsFile      string
	refsJSON      bool
	refsHotspot   bool
	refsLimit     int
	refsNoRefresh bool
	refsWorkspace string
)

// WorkspaceRefRow wraps a RefRow with the originating repo ID for
// workspace-scoped fan-out results.
type WorkspaceRefRow struct {
	indexer.RefRow
	RepoID string `json:"repo_id"`
}

var refsCmd = &cobra.Command{
	Use:   "refs [root]",
	Short: "Search cross-references in the index",
	Long: `Query the refs table for [root]. Use --caller, --callee, or --file to filter.
With no flags, all indexed call sites are returned.
Use --hotspot to print the top-N most-called symbols ranked by inbound call count.
When --workspace is set, [root] is ignored and the query fans out across all repos in the workspace.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRefs,
}

func runRefs(cmd *cobra.Command, args []string) error {
	if refsWorkspace != "" {
		return runRefsWorkspace(cmd, args)
	}
	if len(args) == 0 {
		return fmt.Errorf("requires a [root] argument when --workspace is not set")
	}
	return runRefsSingle(cmd, args[0])
}

func runRefsSingle(cmd *cobra.Command, root string) error {
	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	if !refsNoRefresh {
		if _, err := indexer.AutoRefresh(root, db, RefreshThreshold); err != nil {
			return fmt.Errorf("auto-refresh: %w", err)
		}
	}

	if refsHotspot {
		entries, err := indexer.HotspotSymbols(db, refsLimit)
		if err != nil {
			return fmt.Errorf("hotspot query failed: %w", err)
		}

		if refsJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
		}

		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no call refs found in index")
			return nil
		}

		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "%-5s  %-40s  %10s  %s\n", "rank", "callee", "call_count", "file")
		fmt.Fprintf(w, "%-5s  %-40s  %10s  %s\n", "----", "------", "----------", "----")
		for i, e := range entries {
			file := e.FilePath
			if file == "" {
				file = "(external)"
			}
			fmt.Fprintf(w, "%-5d  %-40s  %10d  %s\n", i+1, e.CalleeName, e.CallCount, file)
		}
		return nil
	}

	q := indexer.RefQuery{
		CallerName: refsCaller,
		CalleeName: refsCallee,
		CallerFile: refsFile,
	}

	rows, err := indexer.SearchRefs(db, q)
	if err != nil {
		return fmt.Errorf("refs query failed: %w", err)
	}

	if refsJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no refs found")
		return nil
	}

	for _, r := range rows {
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s → %-20s line %d\n",
			r.CallerFile, r.CallerName, r.CalleeName, r.Line)
	}

	return nil
}

func runRefsWorkspace(cmd *cobra.Command, _ []string) error {
	// Early-return guard: --hotspot and --workspace are mutually exclusive
	if refsHotspot {
		return fmt.Errorf("--hotspot and --workspace are mutually exclusive")
	}

	wsDB, err := workspace.OpenWorkspace(refsWorkspace)
	if err != nil {
		return fmt.Errorf("cannot open workspace %q: %w", refsWorkspace, err)
	}
	defer wsDB.Close()

	repos, err := workspace.ListRepositories(wsDB)
	if err != nil {
		return fmt.Errorf("cannot list workspace repositories: %w", err)
	}

	q := indexer.RefQuery{
		CallerName: refsCaller,
		CalleeName: refsCallee,
		CallerFile: refsFile,
	}

	var all []WorkspaceRefRow
	for _, repo := range repos {
		repoRows, err := searchRepoRefs(repo, q, refsNoRefresh)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping repo %s (%s): %v\n", repo.ID, repo.Path, err)
			continue
		}
		for _, r := range repoRows {
			all = append(all, WorkspaceRefRow{RefRow: r, RepoID: repo.ID})
		}
	}

	if refsJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(all)
	}

	if len(all) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no refs found")
		return nil
	}

	for _, r := range all {
		fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-40s %-20s → %-20s line %d\n",
			r.RepoID, r.CallerFile, r.CallerName, r.CalleeName, r.Line)
	}

	return nil
}

// searchRepoRefs opens the index for a single repo, optionally auto-refreshes,
// and runs the ref query. The db is closed before returning.
// noRefresh controls whether auto-refresh is skipped; when true, only searches without refreshing.
func searchRepoRefs(repo workspace.Repository, q indexer.RefQuery, noRefresh bool) ([]indexer.RefRow, error) {
	db, err := indexer.OpenIndex(repo.Path)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer db.Close()

	if !noRefresh {
		if _, err := indexer.AutoRefresh(repo.Path, db, RefreshThreshold); err != nil {
			return nil, fmt.Errorf("auto-refresh: %w", err)
		}
	}

	rows, err := indexer.SearchRefs(db, q)
	if err != nil {
		return nil, fmt.Errorf("search refs: %w", err)
	}
	return rows, nil
}
