package indexer

// callers.go — FindCallers: reverse lookup of all call sites targeting a symbol.

import (
	"database/sql"
	"fmt"
)

// CallerRow is a single result from FindCallers: one call site that invokes
// the requested symbol.
type CallerRow struct {
	CallerFile string `json:"caller_file"` // file that contains the call
	CallerName string `json:"caller_name"` // enclosing symbol (empty = file scope)
	CalleeName string `json:"callee_name"` // the symbol being called (== queried name)
	Line       int    `json:"line"`        // 1-based line of the call site
}

// FindCallers returns every recorded call site that invokes calleeName.
// Results are ordered by caller_file, then line for deterministic output.
func FindCallers(db *sql.DB, calleeName string) ([]CallerRow, error) {
	if calleeName == "" {
		return nil, fmt.Errorf("FindCallers: callee name must not be empty")
	}

	rows, err := db.Query(`
		SELECT caller_file, COALESCE(caller_name, ''), callee_name, line
		FROM refs
		WHERE callee_name = ?
		ORDER BY caller_file, line`,
		calleeName,
	)
	if err != nil {
		return nil, fmt.Errorf("FindCallers query: %w", err)
	}
	defer rows.Close()

	var results []CallerRow
	for rows.Next() {
		var r CallerRow
		if err := rows.Scan(&r.CallerFile, &r.CallerName, &r.CalleeName, &r.Line); err != nil {
			return nil, fmt.Errorf("FindCallers scan: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FindCallers rows: %w", err)
	}

	return results, nil
}
