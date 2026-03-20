package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/spf13/cobra"
)

var jsonOutput bool
var indexRebuild bool

var indexCmd = &cobra.Command{
	Use:   "index <path>",
	Short: "Index a repository",
	Long:  `Walk <path>, parse all supported source files, and persist symbols to the SQLite index.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runIndex,
}

func runIndex(cmd *cobra.Command, args []string) error {
	root := args[0]

	if indexRebuild {
		if err := indexer.DropIndex(root); err != nil {
			return fmt.Errorf("cannot drop index: %w", err)
		}
	}

	db, err := indexer.OpenIndex(root)
	if err != nil {
		var mismatch *indexer.SchemaMismatchError
		if errors.As(err, &mismatch) {
			return fmt.Errorf(
				"index schema has changed (v%d → v%d): run `mimir index --rebuild %s` to recreate it",
				mismatch.Stored, mismatch.Current, root,
			)
		}
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	stats, err := indexer.Run(root, db)
	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(stats)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"added %d  updated %d  unchanged %d  removed %d  errors %d\n",
		stats.Added, stats.Updated, stats.Unchanged, stats.Removed, stats.Errors,
	)

	for _, fe := range stats.FileErrors {
		fmt.Fprintf(cmd.ErrOrStderr(), "  error: %s: %v\n", fe.Path, fe.Err)
	}

	return nil
}
