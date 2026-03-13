package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/utahjezz/mimir/pkg/indexer"
)

var (
	treeJSON  bool
	treeFiles bool
	treeDepth int
)

var treeCmd = &cobra.Command{
	Use:   "tree <root>",
	Short: "Show the indexed file tree for a repository",
	Long: `Print a directory tree derived from the index, with file and symbol counts per directory.
Use --depth N to limit output to the top N levels (useful for large repositories).`,
	Args: cobra.ExactArgs(1),
	RunE: runTree,
}

func runTree(cmd *cobra.Command, args []string) error {
	root := args[0]

	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	tree, err := indexer.GetFileTree(db)
	if err != nil {
		return fmt.Errorf("tree query failed: %w", err)
	}

	if treeJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(tree)
	}

	w := cmd.OutOrStdout()
	for _, node := range indexer.FlattenTree(tree) {
		indent := ""
		depth := 0
		if node.Path != "." {
			parts := splitPath(node.Path)
			depth = len(parts)
			// Skip nodes deeper than --depth (0 means unlimited).
			if treeDepth > 0 && depth > treeDepth {
				continue
			}
			for i := 0; i < depth-1; i++ {
				indent += "  "
			}
		}

		langs := ""
		for lang, count := range node.Languages {
			if langs != "" {
				langs += ", "
			}
			langs += fmt.Sprintf("%s:%d", lang, count)
		}

		fmt.Fprintf(w, "%s%-40s  %3d files  %5d symbols  [%s]\n",
			indent, node.Path+"/", node.FileCount, node.SymbolCount, langs)

		if treeFiles {
			for _, f := range node.Files {
				fmt.Fprintf(w, "%s  %-38s  %5d symbols  (%s)\n",
					indent, f.Path, f.SymbolCount, f.Language)
			}
		}
	}

	return nil
}

// splitPath splits a slash-separated path into its components.
func splitPath(p string) []string {
	var parts []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}
