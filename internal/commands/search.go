package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	searchName      string
	searchLike      string
	searchFuzzy     string
	searchType      string
	searchFile      string
	searchJSON      bool
	searchNoRefresh bool
	searchWorkspace string
)

// WorkspaceSymbolRow wraps a SymbolRow with the originating repo ID for
// workspace-scoped fan-out results.
type WorkspaceSymbolRow struct {
	indexer.SymbolRow
	RepoID string `json:"repo_id"`
}

var searchCmd = &cobra.Command{
	Use:   "search [root]",
	Short: "Search indexed symbols with optional filters",
	Long: `Query the symbol index for [root] using any combination of filters.
With no flags, all indexed symbols are returned.
When --workspace is set, [root] is ignored and the search fans out across all repos in the workspace.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSearch,
}

func runSearch(cmd *cobra.Command, args []string) error {
	if searchWorkspace != "" {
		return runSearchWorkspace(cmd, args)
	}
	if len(args) == 0 {
		return fmt.Errorf("requires a [root] argument when --workspace is not set")
	}
	return runSearchSingle(cmd, args[0])
}

func runSearchSingle(cmd *cobra.Command, root string) error {
	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	if !searchNoRefresh {
		if _, err := indexer.AutoRefresh(root, db, RefreshThreshold); err != nil {
			return fmt.Errorf("auto-refresh: %w", err)
		}
	}

	q := indexer.SearchQuery{
		Name:      searchName,
		NameLike:  searchLike,
		FuzzyName: searchFuzzy,
		Type:      indexer.SymbolType(searchType),
		FilePath:  searchFile,
	}

	results, err := indexer.SearchSymbols(db, q)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if searchJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
	}

	if len(results) == 0 {
		if searchFuzzy != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "no FTS5 matches for %q — try --like or --name for exact/prefix search\n", searchFuzzy)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "no symbols found")
		}
		return nil
	}

	for _, r := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-40s %-40s %d-%d\n",
			r.Type, r.Name, r.FilePath, r.StartLine, r.EndLine)
	}

	return nil
}

func runSearchWorkspace(cmd *cobra.Command, _ []string) error {
	wsDB, err := workspace.OpenWorkspace(searchWorkspace)
	if err != nil {
		return fmt.Errorf("cannot open workspace %q: %w", searchWorkspace, err)
	}
	defer wsDB.Close()

	repos, err := workspace.ListRepositories(wsDB)
	if err != nil {
		return fmt.Errorf("cannot list workspace repositories: %w", err)
	}

	q := indexer.SearchQuery{
		Name:      searchName,
		NameLike:  searchLike,
		FuzzyName: searchFuzzy,
		Type:      indexer.SymbolType(searchType),
		FilePath:  searchFile,
	}

	var all []WorkspaceSymbolRow
	for _, repo := range repos {
		repoResults, err := searchRepoSymbols(repo, q, searchNoRefresh)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping repo %s (%s): %v\n", repo.ID, repo.Path, err)
			continue
		}
		for _, r := range repoResults {
			all = append(all, WorkspaceSymbolRow{SymbolRow: r, RepoID: repo.ID})
		}
	}

	if searchJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(all)
	}

	if len(all) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no symbols found")
		return nil
	}

	for _, r := range all {
		fmt.Fprintf(cmd.OutOrStdout(), "%-28s %-12s %-40s %-40s %d-%d\n",
			r.RepoID, r.Type, r.Name, r.FilePath, r.StartLine, r.EndLine)
	}

	return nil
}

// searchRepoSymbols opens the index for a single repo, optionally auto-refreshes,
// and runs the query. The db is closed before returning.
// noRefresh controls whether auto-refresh is skipped; when true, only searches without refreshing.
func searchRepoSymbols(repo workspace.Repository, q indexer.SearchQuery, noRefresh bool) ([]indexer.SymbolRow, error) {
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

	results, err := indexer.SearchSymbols(db, q)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return results, nil
}
