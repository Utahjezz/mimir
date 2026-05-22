package indexer

import (
	"database/sql"
	"fmt"
	"sort"
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

	// FilePath filters results to symbols in files whose path contains this substring.
	FilePath string

	// Limit caps the number of rows returned. Zero means no limit.
	Limit int
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
	// LastIndexByte so that FQN inputs like "A.B.C" split into parent="A.B", name="C"
	// rather than parent="A", name="B.C" (which the old IndexByte produced).
	if dot := strings.LastIndexByte(q.Name, '.'); dot >= 0 {
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
	if dot := strings.LastIndexByte(q.NameLike, '.'); dot >= 0 {
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
		conds = append(conds, "INSTR(file_path, ?) > 0")
		args = append(args, q.FilePath)
	}

	query := base
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY file_path, start_line"
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	return scanSymbolRows(db.Query(query, args...))
}

// searchSymbolsFTS uses the FTS5 virtual table for fuzzy name matching.
// Additional Type and FilePath filters are applied as SQL predicates on the join.
//
// When the query contains no FTS5 operators (* " : ^), each query word is split
// via splitIdentifier and matched against the name_tokens column so that, for
// example, "user address" matches getUserPrimaryAddress. A trailing *
// is appended to every token so prefix matching still works (e.g. "addr").
//
// When the query already contains FTS5 operators the raw query is forwarded
// unchanged to preserve power-user syntax.
func searchSymbolsFTS(db *sql.DB, q SearchQuery) ([]SymbolRow, error) {
	if hasFTSOperators(q.FuzzyName) {
		return searchSymbolsRawFTS(db, q)
	}

	words := tokenizeQuery(q.FuzzyName)
	if len(words) == 0 {
		return []SymbolRow{}, nil
	}

	return searchSymbolsSoftFTS(db, q, words)
}

func hasFTSOperators(query string) bool {
	return strings.ContainsAny(query, "*\":^")
}

func searchSymbolsRawFTS(db *sql.DB, q SearchQuery) ([]SymbolRow, error) {
	query, args := buildFTSBaseQuery(q, q.FuzzyName)
	query += " ORDER BY f.rank"
	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	return scanSymbolRows(db.Query(query, args...))
}

func searchSymbolsSoftFTS(db *sql.DB, q SearchQuery, words []string) ([]SymbolRow, error) {
	query, args := buildFTSBaseQuery(q, buildSoftFuzzyQuery(words))
	query = strings.Replace(query,
		"SELECT s.file_path, s.name, s.type, s.start_line, s.end_line, s.parent",
		"SELECT s.file_path, s.name, s.type, s.start_line, s.end_line, s.parent, s.name_tokens, s.body_snippet, f.rank",
		1,
	)
	query += " ORDER BY f.rank"

	candidates, err := scanFuzzyCandidates(db.Query(query, args...))
	if err != nil {
		return nil, err
	}

	threshold := minRequiredFuzzyMatches(len(words))
	filtered := make([]fuzzyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		metrics := fuzzyMatchMetrics(candidate.nameTokens, candidate.bodySnippet, words)
		total := metrics.totalMatches
		if total < threshold {
			continue
		}
		candidate.totalMatches = metrics.totalMatches
		candidate.nameMatches = metrics.nameMatches
		candidate.bodyOnlyMatches = metrics.bodyOnlyMatches
		candidate.extraNameTokens = metrics.extraNameTokens
		filtered = append(filtered, candidate)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]

		if left.totalMatches != right.totalMatches {
			return left.totalMatches > right.totalMatches
		}
		if left.nameMatches != right.nameMatches {
			return left.nameMatches > right.nameMatches
		}
		if left.bodyOnlyMatches != right.bodyOnlyMatches {
			return left.bodyOnlyMatches < right.bodyOnlyMatches
		}
		if left.extraNameTokens != right.extraNameTokens {
			return left.extraNameTokens < right.extraNameTokens
		}
		if left.rank != right.rank {
			return left.rank < right.rank
		}
		if left.FilePath != right.FilePath {
			return left.FilePath < right.FilePath
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		return left.Name < right.Name
	})

	results := make([]SymbolRow, 0, len(filtered))
	for _, candidate := range filtered {
		results = append(results, candidate.SymbolRow)
	}
	if q.Limit > 0 && len(results) > q.Limit {
		results = results[:q.Limit]
	}
	return results, nil
}

func buildFTSBaseQuery(q SearchQuery, ftsQuery string) (string, []any) {
	var conds []string
	args := []any{ftsQuery}

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
		conds = append(conds, "INSTR(s.file_path, ?) > 0")
		args = append(args, q.FilePath)
	}

	query := `SELECT s.file_path, s.name, s.type, s.start_line, s.end_line, s.parent
	          FROM symbols s
	          JOIN symbols_fts f ON f.rowid = s.id
	          WHERE symbols_fts MATCH ?`
	if len(conds) > 0 {
		query += " AND " + strings.Join(conds, " AND ")
	}
	return query, args
}

func buildSoftFuzzyQuery(words []string) string {
	parts := make([]string, len(words))
	for i, word := range words {
		parts[i] = "(name_tokens : " + word + "* OR body_snippet : " + word + "*)"
	}
	return strings.Join(parts, " OR ")
}

func minRequiredFuzzyMatches(tokenCount int) int {
	switch {
	case tokenCount <= 0:
		return 0
	case tokenCount <= 2:
		return tokenCount
	case tokenCount <= 4:
		return 2
	default:
		return (6*tokenCount + 9) / 10
	}
}

type fuzzyMetrics struct {
	totalMatches    int
	nameMatches     int
	bodyOnlyMatches int
	extraNameTokens int
}

func fuzzyMatchMetrics(nameTokens, bodySnippet string, words []string) fuzzyMetrics {
	nameParts := strings.Fields(strings.ToLower(nameTokens))
	bodyParts := strings.Fields(strings.ToLower(bodySnippet))

	totalMatches := 0
	nameMatches := 0
	bodyOnlyMatches := 0
	for _, word := range words {
		matchedName := hasPrefixTokenMatch(nameParts, word)
		matchedBody := hasPrefixTokenMatch(bodyParts, word)
		if matchedName || matchedBody {
			totalMatches++
		}
		if matchedName {
			nameMatches++
		} else if matchedBody {
			bodyOnlyMatches++
		}
	}

	return fuzzyMetrics{
		totalMatches:    totalMatches,
		nameMatches:     nameMatches,
		bodyOnlyMatches: bodyOnlyMatches,
		extraNameTokens: extraNameTokenCount(nameParts, words),
	}
}

func extraNameTokenCount(nameParts, words []string) int {
	matched := 0
	for _, part := range nameParts {
		if hasPrefixTokenMatch(words, part) || hasQueryWordPrefix(part, words) {
			matched++
		}
	}
	extra := len(nameParts) - matched
	if extra < 0 {
		return 0
	}
	return extra
}

func hasQueryWordPrefix(part string, words []string) bool {
	for _, word := range words {
		if strings.HasPrefix(part, word) || strings.HasPrefix(word, part) {
			return true
		}
	}
	return false
}

func hasPrefixTokenMatch(parts []string, word string) bool {
	for _, part := range parts {
		if strings.HasPrefix(part, word) {
			return true
		}
	}
	return false
}

type fuzzyCandidate struct {
	SymbolRow
	nameTokens   string
	bodySnippet  string
	rank         float64
	totalMatches int
	nameMatches  int
	bodyOnlyMatches int
	extraNameTokens int
}

func scanFuzzyCandidates(rows *sql.Rows, err error) ([]fuzzyCandidate, error) {
	if err != nil {
		return nil, fmt.Errorf("SearchSymbols query: %w", err)
	}
	defer rows.Close()

	type dedupKey struct {
		file      string
		name      string
		typ       string
		startLine int
	}

	seen := make(map[dedupKey]struct{})
	results := make([]fuzzyCandidate, 0)
	for rows.Next() {
		var candidate fuzzyCandidate
		var typ string
		if err := rows.Scan(
			&candidate.FilePath,
			&candidate.Name,
			&typ,
			&candidate.StartLine,
			&candidate.EndLine,
			&candidate.Parent,
			&candidate.nameTokens,
			&candidate.bodySnippet,
			&candidate.rank,
		); err != nil {
			return nil, fmt.Errorf("SearchSymbols scan: %w", err)
		}
		candidate.Type = SymbolType(typ)

		key := dedupKey{candidate.FilePath, candidate.Name, typ, candidate.StartLine}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SearchSymbols rows: %w", err)
	}

	return results, nil
}

// scanSymbolRows reads a *sql.Rows result into a []SymbolRow slice.
// It deduplicates rows by (FilePath, Name, Type, StartLine) to guard against
// pre-existing indexes that were built before the UNIQUE constraint was added.
func scanSymbolRows(rows *sql.Rows, err error) ([]SymbolRow, error) {
	if err != nil {
		return nil, fmt.Errorf("SearchSymbols query: %w", err)
	}
	defer rows.Close()

	type dedupKey struct {
		file      string
		name      string
		typ       string
		startLine int
	}
	seen := make(map[dedupKey]struct{})
	results := []SymbolRow{}
	for rows.Next() {
		var r SymbolRow
		var typ string
		if err := rows.Scan(&r.FilePath, &r.Name, &typ, &r.StartLine, &r.EndLine, &r.Parent); err != nil {
			return nil, fmt.Errorf("SearchSymbols scan: %w", err)
		}
		r.Type = SymbolType(typ)
		key := dedupKey{r.FilePath, r.Name, typ, r.StartLine}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SearchSymbols rows: %w", err)
	}

	return results, nil
}
