package commands

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	workspaceLinkSrcFile string
	workspaceLinkDstFile string
	workspaceLinkNote    string
	workspaceLinkMeta    []string
)

var workspaceLinkCmd = &cobra.Command{
	Use:   "link <src-repo-id> <src-symbol> <dst-repo-id> <dst-symbol> [workspace]",
	Short: "Declare a cross-repo symbol link",
	Long: `Declare a link from a symbol in one repository to a symbol in another.

Both <src-repo-id> and <dst-repo-id> are the repo IDs shown by
"mimir workspace add" and "mimir workspace show" (e.g. backend-a1b2c3d4).

Both symbols are validated against their repo indexes. If a symbol name is
ambiguous (multiple matches), the command lists all candidates and asks you to
re-run with --src-file or --dst-file to disambiguate.`,
	Args: cobra.RangeArgs(4, 5),
	RunE: runWorkspaceLink,
}

func runWorkspaceLink(cmd *cobra.Command, args []string) error {
	srcRepoID := args[0]
	srcSymbolName := args[1]
	dstRepoID := args[2]
	dstSymbolName := args[3]

	workspaceName, err := resolveWorkspaceName(args, 4)
	if err != nil {
		return err
	}

	db, err := workspace.OpenWorkspace(workspaceName)
	if err != nil {
		return fmt.Errorf("cannot open workspace: %w", err)
	}
	defer db.Close()

	// Resolve paths from repo IDs so we can open their indexes.
	srcRepoPath, err := repoPathFromID(db, srcRepoID)
	if err != nil {
		return fmt.Errorf("src-repo: %w", err)
	}
	dstRepoPath, err := repoPathFromID(db, dstRepoID)
	if err != nil {
		return fmt.Errorf("dst-repo: %w", err)
	}

	// Validate src symbol.
	srcFile, err := resolveSymbol(srcRepoPath, srcSymbolName, workspaceLinkSrcFile, "src")
	if err != nil {
		return err
	}

	// Validate dst symbol.
	dstFile, err := resolveSymbol(dstRepoPath, dstSymbolName, workspaceLinkDstFile, "dst")
	if err != nil {
		return err
	}

	// Parse --meta key=value pairs.
	metaPairs, err := parseMetaFlags(workspaceLinkMeta)
	if err != nil {
		return err
	}

	// Persist the link.
	linkID, err := workspace.CreateLink(db, srcRepoID, srcSymbolName, srcFile, dstRepoID, dstSymbolName, dstFile, workspaceLinkNote)
	if err != nil {
		return fmt.Errorf("cannot create link: %w", err)
	}

	for k, v := range metaPairs {
		if err := workspace.SetLinkMeta(db, linkID, k, v); err != nil {
			return fmt.Errorf("cannot set link meta %q: %w", k, err)
		}
	}

	cmd.Printf("Link #%d created: %s (%s) → %s (%s)\n",
		linkID, srcSymbolName, srcRepoID, dstSymbolName, dstRepoID)
	return nil
}

// repoPathFromID looks up repoID in the workspace repositories table and
// returns its stored filesystem path. Returns an error if the repo is not registered.
func repoPathFromID(db *sql.DB, repoID string) (string, error) {
	repos, err := workspace.ListRepositories(db)
	if err != nil {
		return "", fmt.Errorf("cannot list repositories: %w", err)
	}
	for _, r := range repos {
		if r.ID == repoID {
			return r.Path, nil
		}
	}
	return "", fmt.Errorf("repository %q is not registered in this workspace — if the repo is already registered, pass its ID (e.g. myrepo-a1b2c3d4) as shown by `mimir workspace show`, not its path", repoID)
}

// resolveSymbol opens the repo index at repoPath, searches for symbolName,
// and returns the matched file path. fileHint narrows results when provided.
// side is "src" or "dst" and is used only in error messages.
func resolveSymbol(repoPath, symbolName, fileHint, side string) (string, error) {
	repoDB, err := indexer.OpenIndex(repoPath)
	if err != nil {
		return "", fmt.Errorf("cannot open %s repo index at %q: %w", side, repoPath, err)
	}
	defer repoDB.Close()

	rows, err := indexer.SearchSymbols(repoDB, indexer.SearchQuery{Name: symbolName})
	if err != nil {
		return "", fmt.Errorf("cannot search %s symbols: %w", side, err)
	}

	// Apply file hint filter if provided.
	if fileHint != "" {
		filtered := rows[:0]
		for _, r := range rows {
			if strings.HasSuffix(r.FilePath, fileHint) {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	switch len(rows) {
	case 0:
		if fileHint != "" {
			return "", fmt.Errorf("symbol %q not found in %s repo %q (with --%s-file %q)", symbolName, side, repoPath, side, fileHint)
		}
		return "", fmt.Errorf("symbol %q not found in %s repo %q", symbolName, side, repoPath)
	case 1:
		return rows[0].FilePath, nil
	default:
		var sb strings.Builder
		fmt.Fprintf(&sb, "symbol %q is ambiguous in %s repo %q — %d matches:\n", symbolName, side, repoPath, len(rows))
		for _, r := range rows {
			fmt.Fprintf(&sb, "  %s:%d  (%s)\n", r.FilePath, r.StartLine, r.Type)
		}
		fmt.Fprintf(&sb, "re-run with --%s-file <path> to disambiguate", side)
		return "", fmt.Errorf("%s", sb.String())
	}
}

// parseMetaFlags parses a slice of "key=value" strings into a map.
func parseMetaFlags(pairs []string) (map[string]string, error) {
	meta := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid --meta value %q: expected format key=value", p)
		}
		meta[k] = v
	}
	return meta, nil
}
