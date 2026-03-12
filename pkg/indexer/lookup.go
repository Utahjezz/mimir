package indexer

import (
	"database/sql"
	"fmt"
	"strings"
)

// SearchQuery defines optional filters for SearchSymbols.
// All non-zero fields are combined with AND.
//
// Dot notation in Name/NameLike is parsed automatically:
//   - "Class.method"  → exact parent + exact name
//   - "Class.*"       → exact parent, any name
//   - "*.method"      → any parent (non-empty), exact name
//   - "Class.meth"    → exact parent + name prefix (when using NameLike)
type SearchQuery struct {
	// Name matches the symbol name exactly (case-sensitive).
	Name string

	// NameLike matches symbol names using SQL LIKE with a trailing wildcard
	// (e.g. "Foo" becomes "Foo%"). Ignored when Name is set.
	NameLike string

	// FuzzyName performs an FTS5 MATCH query over symbol names.
	// Supports prefix queries (e.g. "Exec*"), multi-token ("execute async"),
	// and phrase search ("\"execute async\"").
	// Ignored when Name is set. Takes precedence over NameLike.
	FuzzyName string

	// Parent matches the enclosing class/struct/interface name exactly.
	// Use "*" to match any non-empty parent (i.e. any method of any class).
	Parent string

	// Type filters by symbol type (e.g. Function, Method, Class).
	// Zero value means no type filter.
	Type SymbolType

	// FilePath filters to symbols belonging to this exact file path.
	FilePath string
}

// ParseDotNotation detects "Parent.Name" syntax in q.Name / q.NameLike and
// splits it into q.Parent + q.Name (or q.NameLike). The original field is
// cleared after splitting. Called automatically by SearchSymbols.
//
// Wildcard rules:
//   - "Class.*"   → Parent="Class",  Name=""  (no name filter)
//   - "*.method"  → Parent="*",      Name="method"
//   - "Class.m"   → if from NameLike: Parent="Class", NameLike="m"
func ParseDotNotation(q SearchQuery) SearchQuery {
	if dot := strings.IndexByte(q.Name, '.'); dot >= 0 {
		parent := q.Name[:dot]
		name := q.Name[dot+1:]
		q.Parent = parent
		if name == "*" {
			q.Name = ""
		} else {
			q.Name = name
		}
		return q
	}
	if dot := strings.IndexByte(q.NameLike, '.'); dot >= 0 {
		parent := q.NameLike[:dot]
		name := q.NameLike[dot+1:]
		q.Parent = parent
		if name == "*" {
			q.NameLike = ""
		} else {
			q.NameLike = name
		}
		return q
	}
	return q
}

// SymbolRow is a query result: a SymbolInfo plus the file it lives in.
type SymbolRow struct {
	SymbolInfo
	FilePath string `json:"file_path"`
}

// SearchSymbols queries the index for symbols matching q.
// All non-zero fields in q are applied as additive AND conditions.
// Dot notation in Name/NameLike is parsed automatically.
// Returns an empty (non-nil) slice when no rows match.
func SearchSymbols(db *sql.DB, q SearchQuery) ([]SymbolRow, error) {
	q = ParseDotNotation(q)

	if q.FuzzyName != "" {
		return searchSymbolsFTS(db, q)
	}
	return searchSymbolsSQL(db, q)
}

// searchSymbolsSQL is the standard WHERE-clause path used when FuzzyName is not set.
func searchSymbolsSQL(db *sql.DB, q SearchQuery) ([]SymbolRow, error) {
	base := `SELECT file_path, name, type, start_line, end_line, parent FROM symbols`

	var conds []string
	var args []any

	if q.Name != "" {
		conds = append(conds, "name = ?")
		args = append(args, q.Name)
	} else if q.NameLike != "" {
		conds = append(conds, "name LIKE ?")
		args = append(args, q.NameLike+"%")
	}

	if q.Parent != "" {
		if q.Parent == "*" {
			conds = append(conds, "parent != ''")
		} else {
			conds = append(conds, "parent = ?")
			args = append(args, q.Parent)
		}
	}

	if q.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, string(q.Type))
	}

	if q.FilePath != "" {
		conds = append(conds, "file_path = ?")
		args = append(args, q.FilePath)
	}

	query := base
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY file_path, start_line"

	return scanSymbolRows(db.Query(query, args...))
}

// searchSymbolsFTS uses the FTS5 virtual table for fuzzy name matching.
// Additional Type and FilePath filters are applied as SQL predicates on the join.
// If the query contains no FTS5 operator (* " :), a trailing * is appended
// automatically so bare tokens behave as prefix searches.
func searchSymbolsFTS(db *sql.DB, q SearchQuery) ([]SymbolRow, error) {
	var conds []string
	var args []any

	// Auto-append * for bare token queries so "serv" matches "serve", "service", etc.
	// Leave the query unchanged if it already contains FTS5 operators.
	ftsQuery := q.FuzzyName
	if !strings.ContainsAny(ftsQuery, "*\":^") {
		ftsQuery += "*"
	}

	// FTS MATCH is the primary filter — name column only.
	args = append(args, ftsQuery)

	if q.Parent != "" {
		if q.Parent == "*" {
			conds = append(conds, "s.parent != ''")
		} else {
			conds = append(conds, "s.parent = ?")
			args = append(args, q.Parent)
		}
	}

	if q.Type != "" {
		conds = append(conds, "s.type = ?")
		args = append(args, string(q.Type))
	}

	if q.FilePath != "" {
		conds = append(conds, "s.file_path = ?")
		args = append(args, q.FilePath)
	}

	query := `SELECT s.file_path, s.name, s.type, s.start_line, s.end_line, s.parent
	          FROM symbols s
	          JOIN symbols_fts f ON f.rowid = s.id
	          WHERE symbols_fts MATCH ?`

	if len(conds) > 0 {
		query += " AND " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY s.file_path, s.start_line"

	return scanSymbolRows(db.Query(query, args...))
}

// scanSymbolRows reads a *sql.Rows result into a []SymbolRow slice.
func scanSymbolRows(rows *sql.Rows, err error) ([]SymbolRow, error) {
	if err != nil {
		return nil, fmt.Errorf("SearchSymbols query: %w", err)
	}
	defer rows.Close()

	results := []SymbolRow{}
	for rows.Next() {
		var r SymbolRow
		var typ string
		if err := rows.Scan(&r.FilePath, &r.Name, &typ, &r.StartLine, &r.EndLine, &r.Parent); err != nil {
			return nil, fmt.Errorf("SearchSymbols scan: %w", err)
		}
		r.Type = SymbolType(typ)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SearchSymbols rows: %w", err)
	}

	return results, nil
}
