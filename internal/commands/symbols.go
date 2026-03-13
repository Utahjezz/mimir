package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var (
	symbolType string
	symbolJSON bool
)

var symbolsCmd = &cobra.Command{
	Use:   "symbols <file>",
	Short: "List all symbols in a file",
	Long:  `Parse <file> and print every extracted symbol with its type and line range.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSymbols,
}

var symbolCmd = &cobra.Command{
	Use:   "symbol <root-or-file> <name>",
	Short: "Show a named symbol and its source",
	Long: `Find the symbol named <name> and print its metadata and full source body.

When <root-or-file> is a directory, the index for that root is queried to resolve
the file path automatically — no need to know which file the symbol lives in.
Multiple matches (e.g. overloaded names) are all printed. Use --type to narrow.

When <root-or-file> is a file path, the file is parsed directly (legacy behaviour).`,
	Args: cobra.ExactArgs(2),
	RunE: runSymbol,
}

func runSymbols(cmd *cobra.Command, args []string) error {
	path := args[0]

	code, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	m := indexer.NewMuncher()
	symbols, err := m.GetSymbols(path, code)
	if err != nil {
		return fmt.Errorf("cannot parse file: %w", err)
	}

	if len(symbols) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no symbols found")
		return nil
	}

	for _, s := range symbols {
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-40s %d-%d\n",
			s.Type, s.Name, s.StartLine, s.EndLine)
	}

	return nil
}

func runSymbol(cmd *cobra.Command, args []string) error {
	rootOrFile, name := args[0], args[1]

	// Detect whether the first argument is a directory (index root) or a file.
	fi, err := os.Stat(rootOrFile)
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", rootOrFile, err)
	}

	if fi.IsDir() {
		return runSymbolFromIndex(cmd, rootOrFile, name)
	}
	return runSymbolFromFile(cmd, rootOrFile, name)
}

// runSymbolFromIndex resolves the symbol via the mimir index, then reads its
// full body from disk. Supports multiple matches, --type, and --json.
func runSymbolFromIndex(cmd *cobra.Command, root, name string) error {
	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	rows, err := indexer.SearchSymbols(db, indexer.SearchQuery{
		Name: name,
		Type: indexer.SymbolType(symbolType),
	})
	if err != nil {
		return fmt.Errorf("index query failed: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Errorf("symbol %q not found in index for %s", name, root)
	}

	m := indexer.NewMuncher()

	type result struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		FilePath  string `json:"file_path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Parent    string `json:"parent,omitempty"`
		Body      string `json:"body"`
	}

	results := make([]result, 0, len(rows))

	for _, r := range rows {
		absPath := filepath.Join(root, r.FilePath)
		body, err := m.GetSymbolContent(absPath, r.StartLine, r.EndLine)
		if err != nil {
			return fmt.Errorf("cannot read body for %q in %s: %w", r.Name, r.FilePath, err)
		}
		results = append(results, result{
			Name:      r.Name,
			Type:      string(r.Type),
			FilePath:  r.FilePath,
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			Parent:    r.Parent,
			Body:      body,
		})
	}

	if symbolJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
	}

	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "---")
		}
		fmt.Fprintf(cmd.OutOrStdout(), "name:  %s\ntype:  %s\nfile:  %s\nlines: %d-%d\n\n",
			r.Name, r.Type, r.FilePath, r.StartLine, r.EndLine)
		fmt.Fprint(cmd.OutOrStdout(), r.Body)
	}

	return nil
}

// runSymbolFromFile is the original behaviour: parse the file directly.
func runSymbolFromFile(cmd *cobra.Command, path, name string) error {
	code, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	m := indexer.NewMuncher()
	sym, err := m.GetSymbol(path, code, name)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "name:  %s\ntype:  %s\nlines: %d-%d\n\n",
		sym.Name, sym.Type, sym.StartLine, sym.EndLine)

	body, err := m.GetSymbolContent(path, sym.StartLine, sym.EndLine)
	if err != nil {
		return fmt.Errorf("cannot read symbol body: %w", err)
	}

	fmt.Fprint(cmd.OutOrStdout(), body)
	return nil
}
