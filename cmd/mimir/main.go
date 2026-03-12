package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/internal/commands"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "mimir",
	Short: "Mimir — language-agnostic code indexer",
	Long: `Mimir indexes source files using tree-sitter, extracts symbol metadata,
and persists the index to SQLite for fast lookup.`,
}

func init() {
	commands.Register(rootCmd)
}
