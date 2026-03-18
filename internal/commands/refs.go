package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
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
)

var refsCmd = &cobra.Command{
	Use:   "refs <root>",
	Short: "Search cross-references in the index",
	Long: `Query the refs table for <root>. Use --caller, --callee, or --file to filter.
With no flags, all indexed call sites are returned.
Use --hotspot to print the top-N most-called symbols ranked by inbound call count.`,
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
