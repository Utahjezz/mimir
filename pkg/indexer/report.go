package indexer

import (
	"database/sql"
	"fmt"
)

// ReportIndex returns a high-level summary of what is stored in the index.
// All data is derived from the existing tables — no schema changes required.
func ReportIndex(db *sql.DB) (RepoReport, error) {
	var report RepoReport

	// --- meta: repo_id, root, git_head ---
	rows, err := db.Query(`SELECT key, value FROM meta WHERE key IN ('repo_id', 'root', 'git_head')`)
	if err != nil {
		return report, fmt.Errorf("ReportIndex meta: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return report, fmt.Errorf("ReportIndex meta scan: %w", err)
		}
		switch k {
		case "repo_id":
			report.RepoID = v
		case "root":
			report.Root = v
		case "git_head":
			report.GitHead = v
		}
	}
	if err := rows.Err(); err != nil {
		return report, fmt.Errorf("ReportIndex meta rows: %w", err)
	}

	// --- indexed_at: most recent file indexing timestamp ---
	if err := db.QueryRow(`SELECT COALESCE(MAX(indexed_at), '') FROM files`).
		Scan(&report.IndexedAt); err != nil {
		return report, fmt.Errorf("ReportIndex indexed_at: %w", err)
	}

	// --- file and symbol totals ---
	if err := db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&report.FileCount); err != nil {
		return report, fmt.Errorf("ReportIndex file count: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM symbols`).Scan(&report.SymbolCount); err != nil {
		return report, fmt.Errorf("ReportIndex symbol count: %w", err)
	}

	// --- language breakdown ---
	langRows, err := db.Query(`
		SELECT f.language, COUNT(DISTINCT f.path), COUNT(s.id)
		FROM files f
		LEFT JOIN symbols s ON s.file_path = f.path
		GROUP BY f.language
		ORDER BY COUNT(s.id) DESC`)
	if err != nil {
		return report, fmt.Errorf("ReportIndex languages: %w", err)
	}
	defer langRows.Close()
	for langRows.Next() {
		var ls LanguageStat
		if err := langRows.Scan(&ls.Language, &ls.FileCount, &ls.SymbolCount); err != nil {
			return report, fmt.Errorf("ReportIndex languages scan: %w", err)
		}
		report.Languages = append(report.Languages, ls)
	}
	if err := langRows.Err(); err != nil {
		return report, fmt.Errorf("ReportIndex languages rows: %w", err)
	}

	// --- symbol type breakdown ---
	typeRows, err := db.Query(`
		SELECT type, COUNT(*)
		FROM symbols
		GROUP BY type
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return report, fmt.Errorf("ReportIndex symbol types: %w", err)
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var st SymbolTypeStat
		if err := typeRows.Scan(&st.Type, &st.Count); err != nil {
			return report, fmt.Errorf("ReportIndex symbol types scan: %w", err)
		}
		report.SymbolTypes = append(report.SymbolTypes, st)
	}
	if err := typeRows.Err(); err != nil {
		return report, fmt.Errorf("ReportIndex symbol types rows: %w", err)
	}

	return report, nil
}
