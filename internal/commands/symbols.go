package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var symbolsCmd = &cobra.Command{
	Use:   "symbols <file>",
	Short: "List all symbols in a file",
	Long:  `Parse <file> and print every extracted symbol with its type and line range.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runSymbols,
}

var symbolCmd = &cobra.Command{
	Use:   "symbol <file> <name>",
	Short: "Show a named symbol and its source",
	Long:  `Parse <file>, find the symbol named <name>, and print its metadata and source body.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runSymbol,
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
	path, name := args[0], args[1]

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
