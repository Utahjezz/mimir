package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var callersJSON bool

var callersCmd = &cobra.Command{
	Use:   "callers <root> <symbol>",
	Short: "Show all call sites that invoke a symbol",
	Long: `Query the index for <root> and list every recorded call site that invokes
<symbol>. Results include the caller file, enclosing function, and line number.`,
	Args: cobra.ExactArgs(2),
	RunE: runCallers,
}

func runCallers(cmd *cobra.Command, args []string) error {
	root, symbol := args[0], args[1]

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	rows, err := indexer.FindCallers(db, symbol)
	if err != nil {
		return fmt.Errorf("callers query failed: %w", err)
	}

	if callersJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "no callers found for %q\n", symbol)
		return nil
	}

	for _, r := range rows {
		caller := r.CallerName
		if caller == "" {
			caller = "<file scope>"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s line %d\n",
			r.CallerFile, caller, r.Line)
	}

	return nil
}
