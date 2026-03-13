package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/internal/commands"
	"github.com/utahjezz/mimir/pkg/indexer"
)

// version is the binary version string. It defaults to "dev" and is
// overridden at build time via:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/mimir
var version = "dev"

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

	rootCmd.Flags().BoolP("version", "v", false, "Print binary version and index schema version")
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		v, _ := cmd.Flags().GetBool("version")
		if v {
			fmt.Fprintf(cmd.OutOrStdout(), "mimir %s (index schema v%d)\n", version, indexer.SchemaVersion())
			return nil
		}
		return cmd.Help()
	}
}
