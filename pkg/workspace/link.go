package workspace

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrLinkNotFound is returned by DeleteLink when no link with the given ID exists.
var ErrLinkNotFound = errors.New("link not found")

// CreateLink inserts a new cross-repo symbol link into the workspace and returns
// the newly assigned link ID.
func CreateLink(db *sql.DB, srcRepoID, srcSymbol, srcFile, dstRepoID, dstSymbol, dstFile, note string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO links (src_repo_id, src_symbol, src_file, dst_repo_id, dst_symbol, dst_file, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		srcRepoID, srcSymbol, srcFile, dstRepoID, dstSymbol, dstFile, note,
	)
	if err != nil {
		return 0, fmt.Errorf("CreateLink: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("CreateLink: last insert id: %w", err)
	}
	return id, nil
}

// SetLinkMeta upserts a key/value metadata entry for the given link.
func SetLinkMeta(db *sql.DB, linkID int64, key, value string) error {
	_, err := db.Exec(
		`INSERT INTO link_meta (link_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(link_id, key) DO UPDATE SET value = excluded.value`,
		linkID, key, value,
	)
	if err != nil {
		return fmt.Errorf("SetLinkMeta: %w", err)
	}
	return nil
}

// LinkQuery holds optional filters for ListLinks.
// Empty string fields are ignored (no filter applied).
type LinkQuery struct {
	SrcRepoID string // filter by source repo ID
	DstRepoID string // filter by destination repo ID
	SrcSymbol string // filter by source symbol name (exact match)
	DstSymbol string // filter by destination symbol name (exact match)
}

// ListLinks returns all links in the workspace matching the given query.
// All fields in the query are optional; empty string means no filter.
// Results are ordered by created_at.
// Each Link's Meta map is populated from the link_meta table.
func ListLinks(db *sql.DB, q LinkQuery) ([]Link, error) {
	query := `SELECT id, src_repo_id, src_symbol, src_file,
	                 dst_repo_id, dst_symbol, dst_file, note, created_at
	          FROM links`
	args := []any{}
	clauses := []string{}

	if q.SrcRepoID != "" {
		clauses = append(clauses, "src_repo_id = ?")
		args = append(args, q.SrcRepoID)
	}
	if q.DstRepoID != "" {
		clauses = append(clauses, "dst_repo_id = ?")
		args = append(args, q.DstRepoID)
	}
	if q.SrcSymbol != "" {
		clauses = append(clauses, "src_symbol = ?")
		args = append(args, q.SrcSymbol)
	}
	if q.DstSymbol != "" {
		clauses = append(clauses, "dst_symbol = ?")
		args = append(args, q.DstSymbol)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY created_at`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListLinks: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(
			&l.ID, &l.SrcRepoID, &l.SrcSymbol, &l.SrcFile,
			&l.DstRepoID, &l.DstSymbol, &l.DstFile, &l.Note, &l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListLinks scan: %w", err)
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListLinks rows: %w", err)
	}

	// Populate Meta maps in a second pass.
	for i := range links {
		meta, err := getLinkMeta(db, links[i].ID)
		if err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			links[i].Meta = meta
		}
	}

	return links, nil
}

// getLinkMeta fetches all key/value metadata entries for a single link ID.
func getLinkMeta(db *sql.DB, linkID int64) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM link_meta WHERE link_id = ? ORDER BY key`, linkID)
	if err != nil {
		return nil, fmt.Errorf("getLinkMeta: %w", err)
	}
	defer rows.Close()

	meta := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("getLinkMeta scan: %w", err)
		}
		meta[k] = v
	}
	return meta, rows.Err()
}

// DeleteLink removes the link with the given ID.
// Returns ErrLinkNotFound when no link with that ID exists.
func DeleteLink(db *sql.DB, id int64) error {
	res, err := db.Exec(`DELETE FROM links WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("DeleteLink: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteLink rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: #%d", ErrLinkNotFound, id)
	}
	return nil
}
