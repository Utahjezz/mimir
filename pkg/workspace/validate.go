package workspace

import (
	"database/sql"
	"fmt"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// ValidateLink checks that the symbols and file paths in a link still exist
// in their respective repositories. It opens each repo's index and searches
// for the symbols to verify they are present and located at the recorded paths.
//
// Returns a populated ValidationResult with the results. The original Link
// data is preserved in result.Link.
func ValidateLink(db *sql.DB, link *Link) (*ValidationResult, error) {
	result := &ValidationResult{
		Link: *link,
	}

	// Look up repository paths.
	srcRepoPath, err := repoPathFromID(db, link.SrcRepoID)
	if err != nil {
		result.Link.SrcError = strPtr(fmt.Sprintf("cannot resolve src repo: %v", err))
	} else {
		srcActual, srcErr := validateSymbolInRepo(srcRepoPath, link.SrcSymbol, link.SrcFile)
		result.Link.SrcActualFile = strPtr(srcActual)
		result.Link.SrcError = strPtr(srcErr)
		result.Link.SrcValid = boolPtr(srcErr == "")
		result.Link.SrcFileValid = boolPtr(srcErr == "" && srcActual == link.SrcFile)
	}

	dstRepoPath, err := repoPathFromID(db, link.DstRepoID)
	if err != nil {
		result.Link.DstError = strPtr(fmt.Sprintf("cannot resolve dst repo: %v", err))
	} else {
		dstActual, dstErr := validateSymbolInRepo(dstRepoPath, link.DstSymbol, link.DstFile)
		result.Link.DstActualFile = strPtr(dstActual)
		result.Link.DstError = strPtr(dstErr)
		result.Link.DstValid = boolPtr(dstErr == "")
		result.Link.DstFileValid = boolPtr(dstErr == "" && dstActual == link.DstFile)
	}

	return result, nil
}

// repoPathFromID looks up repoID in the workspace repositories table and
// returns its stored filesystem path.
func repoPathFromID(db *sql.DB, repoID string) (string, error) {
	repos, err := ListRepositories(db)
	if err != nil {
		return "", fmt.Errorf("cannot list repositories: %w", err)
	}
	for _, r := range repos {
		if r.ID == repoID {
			return r.Path, nil
		}
	}
	return "", fmt.Errorf("repository %q is not registered in this workspace", repoID)
}

// validateSymbolInRepo opens the index for a repository and searches for a symbol
// by name. It returns the actual file path where the symbol was found and an
// error string (empty if successful).
func validateSymbolInRepo(repoPath, symbolName, recordedFile string) (string, string) {
	repoDB, indexErr := indexer.OpenIndex(repoPath)
	if indexErr != nil {
		return "", fmt.Sprintf("cannot open repo index at %q: %v", repoPath, indexErr)
	}
	defer repoDB.Close()

	rows, searchErr := indexer.SearchSymbols(repoDB, indexer.SearchQuery{Name: symbolName})
	if searchErr != nil {
		return "", fmt.Sprintf("cannot search symbols in repo %q: %v", repoPath, searchErr)
	}

	if len(rows) == 0 {
		return "", fmt.Sprintf("symbol %q not found in repo", symbolName)
	}

	// Return the first match - the symbol was found.
	// The caller (ValidateLink) will compare this to recordedFile.
	return rows[0].FilePath, ""
}
