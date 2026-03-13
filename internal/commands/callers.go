package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var (
	callersJSON  bool
	callersDepth int
)

var callersCmd = &cobra.Command{
	Use:   "callers <root> <symbol>",
	Short: "Show all call sites that invoke a symbol",
	Long: `Query the index for <root> and list every recorded call site that invokes
<symbol>. Results include the caller file, enclosing function, and line number.

Use --depth N to recursively trace callers-of-callers up to N levels deep.
Default is --depth 2, which adds one level of context over the flat list.
--depth 1 gives the flat direct-callers list.
--depth 0 means unlimited depth — use with care on widely-called symbols.
Cycles are detected and marked with [cycle].`,
	Args: cobra.ExactArgs(2),
	RunE: runCallers,
}

func runCallers(cmd *cobra.Command, args []string) error {
	root, symbol := args[0], args[1]

	// Warn on unlimited depth before potentially emitting a very large result.
	if callersDepth == 0 && !callersJSON {
		fmt.Fprintln(cmd.ErrOrStderr(), "Warning: unlimited depth — output may be very large for widely-called symbols")
	}

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	if callersDepth == 1 {
		// Fast path: single level, preserve original behaviour.
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

	// Recursive path.
	nodes, err := indexer.FindCallersRecursive(db, symbol, callersDepth)
	if err != nil {
		return fmt.Errorf("callers query failed: %w", err)
	}

	if callersJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(nodes)
	}

	if len(nodes) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "no callers found for %q\n", symbol)
		return nil
	}

	printCallerTree(cmd, nodes, 0)
	return nil
}

// printCallerTree renders the recursive caller tree as an indented text tree.
func printCallerTree(cmd *cobra.Command, nodes []indexer.CallerNode, indent int) {
	prefix := strings.Repeat("  ", indent)
	for _, n := range nodes {
		caller := n.CallerName
		if caller == "" {
			caller = "<file scope>"
		}
		suffix := ""
		if n.Cycle {
			suffix = " [cycle]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s%-40s %-20s line %d%s\n",
			prefix, n.CallerFile, caller, n.Line, suffix)
		if len(n.Callers) > 0 {
			printCallerTree(cmd, n.Callers, indent+1)
		}
	}
}
