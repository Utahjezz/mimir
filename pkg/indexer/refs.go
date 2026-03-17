package indexer

// refs.go — cross-reference query types, SearchRefs, and HotspotSymbols.

import (
	"database/sql"
	"fmt"
	"strings"
)

// RefRow is a single row returned by SearchRefs.
type RefRow struct {
	ID         int    `json:"id"`
	CallerFile string `json:"caller_file"`
	CallerName string `json:"caller_name"`
	CalleeName string `json:"callee_name"`
	Line       int    `json:"line"`
}

// RefQuery holds optional filter fields for SearchRefs.
// Any zero-value field is ignored (no filter applied for that column).
type RefQuery struct {
	CallerFile string // filter by the file that contains the call
	CallerName string // filter by the enclosing symbol name
	CalleeName string // filter by the name of the called function/method
}

// SearchRefs queries the refs table using the filters in q.
// All non-empty fields are AND-ed together. An empty RefQuery returns all rows.
func SearchRefs(db *sql.DB, q RefQuery) ([]RefRow, error) {
	var (
		conds []string
		args  []any
	)

	if q.CallerFile != "" {
		conds = append(conds, "caller_file = ?")
		args = append(args, q.CallerFile)
	}
	if q.CallerName != "" {
		conds = append(conds, "caller_name = ?")
		args = append(args, q.CallerName)
	}
	if q.CalleeName != "" {
		conds = append(conds, "callee_name = ?")
		args = append(args, q.CalleeName)
	}

	query := `SELECT id, caller_file, caller_name, callee_name, line FROM refs`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY caller_file, line"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("SearchRefs query: %w", err)
	}
	defer rows.Close()

	results := make([]RefRow, 0)
	for rows.Next() {
		var r RefRow
		if err := rows.Scan(&r.ID, &r.CallerFile, &r.CallerName, &r.CalleeName, &r.Line); err != nil {
			return nil, fmt.Errorf("SearchRefs scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// HotspotEntry is a single row returned by HotspotSymbols.
type HotspotEntry struct {
	CalleeName string `json:"callee_name"`
	CallCount  int    `json:"call_count"`
	FilePath   string `json:"file_path"` // empty when callee is not in the symbol index (e.g. stdlib)
}

// HotspotSymbols returns the top-limit most-called symbols ranked by inbound
// call count. The file path is resolved via a LEFT JOIN on the symbols table;
// it will be empty for callees not present in the index (external/stdlib calls).
func HotspotSymbols(db *sql.DB, limit int) ([]HotspotEntry, error) {
	const query = `
		SELECT r.callee_name, COUNT(*) AS call_count, COALESCE(MIN(s.file_path), '') AS file_path
		FROM refs r
		LEFT JOIN symbols s ON s.name = r.callee_name
		GROUP BY r.callee_name
		ORDER BY call_count DESC
		LIMIT ?`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("HotspotSymbols query: %w", err)
	}
	defer rows.Close()

	results := make([]HotspotEntry, 0, limit)
	for rows.Next() {
		var e HotspotEntry
		if err := rows.Scan(&e.CalleeName, &e.CallCount, &e.FilePath); err != nil {
			return nil, fmt.Errorf("HotspotSymbols scan: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}
