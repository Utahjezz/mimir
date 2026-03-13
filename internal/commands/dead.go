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
symbols and reduce false positives from public APIs.

NOTE: Dead-code detection uses name-only JOIN matching — a symbol is considered
"live" if any ref row has a callee_name equal to its name, regardless of package
or file. This means common names shared with standard-library functions (e.g.
Open, Close, Error, Read, Write, String) may be incorrectly excluded from
results, producing false negatives. Use --unexported and --file to narrow scope
and reduce noise.`,
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
		checked, err := indexer.CountDeadCandidates(db, q)
		if err != nil {
			checked = 0
		}
		fmt.Fprintf(cmd.OutOrStdout(), "checked %d symbols — none unreachable\n", checked)
	} else {
		for _, s := range symbols {
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-40s %s line %d\n",
				s.Type, s.Name, s.FilePath, s.Line)
		}
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "warning: dead-code uses name-only matching — symbols whose names collide with"+
		" common stdlib names (e.g. Open, Close, Error, Read, Write, String) may be missing from results (false negatives)."+
		" Use --unexported and --file to reduce noise.")

	return nil
}
