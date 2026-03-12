package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var (
	deadType       string
	deadFile       string
	deadUnexported bool
	deadJSON       bool
)

var deadCmd = &cobra.Command{
	Use:   "dead <root>",
	Short: "Find symbols that are never called anywhere in the index",
	Long: `Scan the index for <root> and list functions/methods that have no recorded
callers in the refs table. Use --unexported to limit results to unexported
symbols and reduce false positives from public APIs.`,
	Args: cobra.ExactArgs(1),
	RunE: runDead,
}

func runDead(cmd *cobra.Command, args []string) error {
	root := args[0]

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	q := indexer.DeadCodeQuery{
		Type:           deadType,
		FilePath:       deadFile,
		UnexportedOnly: deadUnexported,
	}

	symbols, err := indexer.FindDeadSymbols(db, q)
	if err != nil {
		return fmt.Errorf("dead code query failed: %w", err)
	}

	if deadJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(symbols)
	}

	if len(symbols) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no dead symbols found")
		return nil
	}

	for _, s := range symbols {
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-40s %s line %d\n",
			s.Type, s.Name, s.FilePath, s.Line)
	}

	return nil
}
