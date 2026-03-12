package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var reportJSON bool

var reportCmd = &cobra.Command{
	Use:   "report <root>",
	Short: "Show a summary report of the index for a repository",
	Long:  `Print file count, symbol count, language breakdown, and symbol type breakdown for <root>.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runReport,
}

func runReport(cmd *cobra.Command, args []string) error {
	root := args[0]

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	report, err := indexer.ReportIndex(db)
	if err != nil {
		return fmt.Errorf("report failed: %w", err)
	}

	if reportJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "repo:      %s\n", report.RepoID)
	fmt.Fprintf(w, "root:      %s\n", report.Root)
	if report.GitHead != "" {
		fmt.Fprintf(w, "git_head:  %s\n", report.GitHead)
	}
	fmt.Fprintf(w, "indexed:   %s\n", report.IndexedAt)
	fmt.Fprintf(w, "files:     %d\n", report.FileCount)
	fmt.Fprintf(w, "symbols:   %d\n", report.SymbolCount)

	fmt.Fprintf(w, "\nlanguages:\n")
	for _, l := range report.Languages {
		fmt.Fprintf(w, "  %-16s %4d files   %6d symbols\n", l.Language, l.FileCount, l.SymbolCount)
	}

	fmt.Fprintf(w, "\nsymbol types:\n")
	for _, s := range report.SymbolTypes {
		fmt.Fprintf(w, "  %-16s %6d\n", s.Type, s.Count)
	}

	return nil
}
