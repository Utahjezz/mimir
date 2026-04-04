package indexer

// imports.go — import-site query types and SearchImports.

import (
	"database/sql"
	"fmt"
	"strings"
)

// ImportRow is a single row returned by SearchImports.
type ImportRow struct {
	FilePath   string `json:"file_path"`
	ImportPath string `json:"import_path"`
	Alias      string `json:"alias,omitempty"`
	Line       int    `json:"line"`
}

// ImportQuery holds optional filter fields for SearchImports.
// Any zero-value field is ignored (no filter applied for that column).
type ImportQuery struct {
	FilePath   string // filter by the file that contains the import
	ImportPath string // filter by the imported module/namespace path
}

// SearchImports queries the imports table using the filters in q.
// All non-empty fields are AND-ed together. An empty ImportQuery returns all rows.
func SearchImports(db *sql.DB, q ImportQuery) ([]ImportRow, error) {
	var (
		conds []string
		args  []any
	)

	if q.FilePath != "" {
		conds = append(conds, "file_path = ?")
		args = append(args, q.FilePath)
	}
	if q.ImportPath != "" {
		conds = append(conds, "import_path = ?")
		args = append(args, q.ImportPath)
	}

	query := `SELECT file_path, import_path, alias, line FROM imports`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY file_path, line"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("SearchImports query: %w", err)
	}
	defer rows.Close()

	results := make([]ImportRow, 0)
	for rows.Next() {
		var r ImportRow
		if err := rows.Scan(&r.FilePath, &r.ImportPath, &r.Alias, &r.Line); err != nil {
			return nil, fmt.Errorf("SearchImports scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
