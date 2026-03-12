package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var (
	refsCaller string
	refsCallee string
	refsFile   string
	refsJSON   bool
)

var refsCmd = &cobra.Command{
	Use:   "refs <root>",
	Short: "Search cross-references in the index",
	Long: `Query the refs table for <root>. Use --caller, --callee, or --file to filter.
With no flags, all indexed call sites are returned.`,
	Args: cobra.ExactArgs(1),
	RunE: runRefs,
}

func runRefs(cmd *cobra.Command, args []string) error {
	root := args[0]

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

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
