package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
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
)

var searchCmd = &cobra.Command{
	Use:   "search <root>",
	Short: "Search indexed symbols with optional filters",
	Long: `Query the symbol index for <root> using any combination of filters.
With no flags, all indexed symbols are returned.`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func runSearch(cmd *cobra.Command, args []string) error {
	root := args[0]

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
