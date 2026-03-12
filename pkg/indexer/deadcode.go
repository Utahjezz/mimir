package indexer

// deadcode.go — FindDeadSymbols: finds symbols that are never called anywhere
// in the index (no matching entry in the refs table as a callee).
//
// A "dead" symbol is a function or method that exists in the symbols table but
// has zero rows in refs where callee_name = symbol.name.
//
// Caveats:
//   - Exported symbols (capitalised first letter, or any name in non-Go languages)
//     may be called from outside the indexed codebase; use --unexported to limit
//     results to unexported names only and reduce false positives.
//   - Only call sites extracted by ExtractCalls (currently Go only) are considered;
//     languages without call extraction will report all their symbols as dead.
//   - Functions invoked by the Go runtime (main, init) and the testing framework
//     (Test*, Benchmark*, Example*, Fuzz*) are excluded automatically because they
//     are never called as ordinary call sites in source code.
//   - Functions passed as values (e.g. RunE: runCmd in cobra commands) are tracked
//     via RefQueries and correctly excluded from dead code results.

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// DeadSymbol is a single result row from FindDeadSymbols.
type DeadSymbol struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
}

// DeadCodeQuery holds optional filters for FindDeadSymbols.
type DeadCodeQuery struct {
	// Type restricts results to a specific symbol type (e.g. "function", "method").
	// Empty means all callable types (function + method).
	Type string

	// FilePath filters results to symbols in files whose path contains this substring.
	FilePath string

	// UnexportedOnly restricts results to symbols whose name starts with a
	// lowercase letter — i.e. unexported in Go convention. Reduces false
	// positives for public APIs.
	UnexportedOnly bool
}

// FindDeadSymbols returns symbols that have no recorded callers in the refs
// table. Only function and method symbols are considered by default; set
// q.Type to override.
//
// The following names are always excluded from results because they are
// invoked by the Go runtime or testing framework, not by ordinary call sites:
//   - main, init (runtime entry points)
//   - Test*, Benchmark*, Example*, Fuzz* (testing framework)
func FindDeadSymbols(db *sql.DB, q DeadCodeQuery) ([]DeadSymbol, error) {
	// Build WHERE clauses dynamically.
	var conditions []string
	var args []any

	// Restrict to callable types unless a specific type is requested.
	if q.Type != "" {
		conditions = append(conditions, "s.type = ?")
		args = append(args, q.Type)
	} else {
		conditions = append(conditions, "s.type IN ('function', 'method')")
	}

	// File path substring filter.
	if q.FilePath != "" {
		conditions = append(conditions, "s.file_path LIKE ?")
		args = append(args, "%"+q.FilePath+"%")
	}

	// Always exclude runtime/test-framework entry points — they are never
	// called as ordinary call sites in source code.
	conditions = append(conditions,
		"s.name NOT IN ('main', 'init')",
		"s.name NOT LIKE 'Test%'",
		"s.name NOT LIKE 'Benchmark%'",
		"s.name NOT LIKE 'Example%'",
		"s.name NOT LIKE 'Fuzz%'",
	)

	where := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT s.name, s.type, s.file_path, s.start_line
		FROM symbols s
		LEFT JOIN refs r ON r.callee_name = s.name
		%s
		  AND r.id IS NULL
		ORDER BY s.file_path, s.start_line`, where)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("FindDeadSymbols query: %w", err)
	}
	defer rows.Close()

	var results []DeadSymbol
	for rows.Next() {
		var d DeadSymbol
		if err := rows.Scan(&d.Name, &d.Type, &d.FilePath, &d.Line); err != nil {
			return nil, fmt.Errorf("FindDeadSymbols scan: %w", err)
		}
		if q.UnexportedOnly && !isUnexported(d.Name) {
			continue
		}
		results = append(results, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FindDeadSymbols rows: %w", err)
	}

	return results, nil
}

// isUnexported reports whether name starts with a lowercase letter — the Go
// convention for unexported identifiers.
func isUnexported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsLower(rune(name[0]))
}
