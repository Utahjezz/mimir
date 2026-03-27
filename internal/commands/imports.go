package commands

// imports.go — "mimir imports" command: query the imports table for a repo or
// across all repos in a workspace.  Agents use this command to resolve
// dependency questions (what does file X import?), identify all consumers of a
// module (which files import "pkg/foo"?), and understand module boundaries.

import (
	"encoding/json"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	importsFile      string
	importsModule    string
	importsJSON      bool
	importsNoRefresh bool
	importsWorkspace string
)

// WorkspaceImportRow wraps an ImportRow with the originating repo ID for
// workspace-scoped fan-out results.
type WorkspaceImportRow struct {
	indexer.ImportRow
	RepoID string `json:"repo_id"`
}

var importsCmd = &cobra.Command{
	Use:   "imports [root]",
	Short: "Query the imports table in the index",
	Long: `Query the imports table for [root].

Use --file to list all imports in a specific source file, or --module to find
every file that imports a particular module/package path.  With no flags, all
indexed import statements are returned.

Agent use-cases:
  • Dependency resolution: which package does symbol X come from?
  • Module boundary analysis: which files depend on an internal package?
  • Refactoring impact: what breaks if "pkg/old" is renamed or removed?

When --workspace is set, [root] is ignored and the query fans out across all
repos in the workspace.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImports,
}

func runImports(cmd *cobra.Command, args []string) error {
	if importsWorkspace != "" {
		return runImportsWorkspace(cmd, args)
	}
	if len(args) == 0 {
		return fmt.Errorf("requires a [root] argument when --workspace is not set")
	}
	return runImportsSingle(cmd, args[0])
}

func runImportsSingle(cmd *cobra.Command, root string) error {
	db, err := indexer.OpenIndex(root)
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	if !importsNoRefresh {
		if _, err := indexer.AutoRefresh(root, db, RefreshThreshold); err != nil {
			return fmt.Errorf("auto-refresh: %w", err)
		}
	}

	q := indexer.ImportQuery{
		FilePath:   importsFile,
		ImportPath: importsModule,
	}

	rows, err := indexer.SearchImports(db, q)
	if err != nil {
		return fmt.Errorf("imports query failed: %w", err)
	}

	if importsJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no imports found")
		return nil
	}

	for _, r := range rows {
		if r.Alias != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %-40s  [%s]  line %d\n",
				r.FilePath, r.ImportPath, r.Alias, r.Line)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %-40s  line %d\n",
				r.FilePath, r.ImportPath, r.Line)
		}
	}
	return nil
}

func runImportsWorkspace(cmd *cobra.Command, _ []string) error {
	wsDB, err := workspace.OpenWorkspace(importsWorkspace)
	if err != nil {
		return fmt.Errorf("cannot open workspace %q: %w", importsWorkspace, err)
	}
	defer wsDB.Close()

	repos, err := workspace.ListRepositories(wsDB)
	if err != nil {
		return fmt.Errorf("cannot list workspace repositories: %w", err)
	}

	q := indexer.ImportQuery{
		FilePath:   importsFile,
		ImportPath: importsModule,
	}

	var all []WorkspaceImportRow
	for _, repo := range repos {
		repoRows, err := searchRepoImports(repo, q, importsNoRefresh)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping repo %s (%s): %v\n", repo.ID, repo.Path, err)
			continue
		}
		for _, r := range repoRows {
			all = append(all, WorkspaceImportRow{ImportRow: r, RepoID: repo.ID})
		}
	}

	if importsJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(all)
	}

	if len(all) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no imports found")
		return nil
	}

	for _, r := range all {
		if r.Alias != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%-28s  %-40s  %-40s  [%s]  line %d\n",
				r.RepoID, r.FilePath, r.ImportPath, r.Alias, r.Line)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%-28s  %-40s  %-40s  line %d\n",
				r.RepoID, r.FilePath, r.ImportPath, r.Line)
		}
	}
	return nil
}

// searchRepoImports opens the index for a single repo, optionally auto-refreshes,
// and runs the import query. The db is closed before returning.
func searchRepoImports(repo workspace.Repository, q indexer.ImportQuery, noRefresh bool) ([]indexer.ImportRow, error) {
	db, err := indexer.OpenIndex(repo.Path)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	defer db.Close()

	if !noRefresh {
		if _, err := indexer.AutoRefresh(repo.Path, db, RefreshThreshold); err != nil {
			return nil, fmt.Errorf("auto-refresh: %w", err)
		}
	}

	rows, err := indexer.SearchImports(db, q)
	if err != nil {
		return nil, fmt.Errorf("search imports: %w", err)
	}
	return rows, nil
}
