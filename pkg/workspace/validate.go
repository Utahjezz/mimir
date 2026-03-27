package workspace

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// ValidateLink checks that the symbols and file paths in a link still exist
// in their respective repositories. It opens each repo's index and searches
// for the symbols to verify they are present and located at the recorded paths.
//
// A non-nil error is returned only for unrecoverable workspace-level failures
// (e.g. the repositories table cannot be queried). Link-level problems — repo
// not registered, symbol not found, ambiguous symbol — are recorded in the
// SrcError/DstError fields of the result and do not cause a non-nil error.
//
// The original Link data is preserved in result.Link.
func ValidateLink(db *sql.DB, link *Link) (*ValidationResult, error) {
	result := &ValidationResult{
		Link: *link,
	}

	// Look up source repository path.
	srcRepoPath, err := repoPathFromID(db, link.SrcRepoID)
	if err != nil {
		if isDBError(err) {
			return nil, fmt.Errorf("cannot query workspace for src repo: %w", unwrapDBError(err))
		}
		result.Link.SrcError = strPtr(err.Error())
	} else {
		srcActual, srcErr := validateSymbolInRepo(srcRepoPath, link.SrcSymbol, link.SrcFile)
		result.Link.SrcActualFile = strPtr(srcActual)
		result.Link.SrcError = strPtr(srcErr)
		result.Link.SrcValid = boolPtr(srcErr == "")
		result.Link.SrcFileValid = boolPtr(srcErr == "" && srcActual == link.SrcFile)
	}

	// Look up destination repository path.
	dstRepoPath, err := repoPathFromID(db, link.DstRepoID)
	if err != nil {
		if isDBError(err) {
			return nil, fmt.Errorf("cannot query workspace for dst repo: %w", unwrapDBError(err))
		}
		result.Link.DstError = strPtr(err.Error())
	} else {
		dstActual, dstErr := validateSymbolInRepo(dstRepoPath, link.DstSymbol, link.DstFile)
		result.Link.DstActualFile = strPtr(dstActual)
		result.Link.DstError = strPtr(dstErr)
		result.Link.DstValid = boolPtr(dstErr == "")
		result.Link.DstFileValid = boolPtr(dstErr == "" && dstActual == link.DstFile)
	}

	return result, nil
}

// dbError wraps a workspace DB failure so ValidateLink can distinguish it from
// a link-level not-found error returned by repoPathFromID.
type dbError struct{ cause error }

func (e *dbError) Error() string { return e.cause.Error() }
func (e *dbError) Unwrap() error { return e.cause }

func isDBError(err error) bool {
	var t *dbError
	return errors.As(err, &t)
}

func unwrapDBError(err error) error {
	var t *dbError
	if errors.As(err, &t) {
		return t.cause
	}
	return err
}

// repoPathFromID looks up repoID in the workspace repositories table and
// returns its stored filesystem path.
// Returns a *dbError if the repositories table cannot be queried (unrecoverable),
// or a plain error if the repo ID is simply not registered (link-level issue).
func repoPathFromID(db *sql.DB, repoID string) (string, error) {
	repos, err := ListRepositories(db)
	if err != nil {
		return "", &dbError{cause: fmt.Errorf("cannot list repositories: %w", err)}
	}
	for _, r := range repos {
		if r.ID == repoID {
			return r.Path, nil
		}
	}
	return "", fmt.Errorf("repository %q is not registered in this workspace", repoID)
}

// validateSymbolInRepo opens the index for a repository and searches for a symbol
// by name, using recordedFile to disambiguate when multiple matches exist.
//
// Resolution order:
//  1. Exact match — a row whose FilePath equals recordedFile → symbol is still
//     at the recorded location, return it as-is.
//  2. Suffix filter — rows whose FilePath has recordedFile as a suffix (handles
//     absolute/relative path differences). If exactly one remains, the symbol
//     moved to that file. If two or more remain, the match is ambiguous.
//  3. No suffix match — symbol exists elsewhere; return the first candidate so
//     the caller can surface a useful "moved to X" message.
//
// Returns (actualFile, errString). errString is empty on success.
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

	// Step 1: exact match — symbol is still at the recorded path.
	for _, r := range rows {
		if r.FilePath == recordedFile {
			return r.FilePath, ""
		}
	}

	// Step 2: suffix filter — narrow to rows whose path ends with recordedFile.
	var suffixed []indexer.SymbolRow
	for _, r := range rows {
		if strings.HasSuffix(r.FilePath, recordedFile) {
			suffixed = append(suffixed, r)
		}
	}

	switch len(suffixed) {
	case 1:
		// Unambiguous move — symbol found at exactly one new location.
		return suffixed[0].FilePath, ""
	case 0:
		// Symbol exists but not near the recorded path; return first candidate
		// so the caller can report where it actually is.
		return rows[0].FilePath, ""
	default:
		// Ambiguous — multiple files match the suffix, cannot determine which
		// is the intended target.
		var sb strings.Builder
		fmt.Fprintf(&sb, "symbol %q is ambiguous — %d matches:\n", symbolName, len(suffixed))
		for _, r := range suffixed {
			fmt.Fprintf(&sb, "  %s\n", r.FilePath)
		}
		sb.WriteString("update the link with an exact file path to disambiguate")
		return "", sb.String()
	}
}
