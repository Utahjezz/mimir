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

// CallerNode is one node in a recursive call graph tree returned by
// FindCallersRecursive. Each node represents a single call site; its Callers
// field holds the next level of callers (who calls this caller), and so on.
type CallerNode struct {
	CallerFile string       `json:"caller_file"`
	CallerName string       `json:"caller_name"` // empty string means file scope
	CalleeName string       `json:"callee_name"`
	Line       int          `json:"line"`
	Cycle      bool         `json:"cycle,omitempty"` // true when this node was already seen — not expanded
	Callers    []CallerNode `json:"callers,omitempty"`
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

// FindCallersRecursive traverses the call graph upward from calleeName up to
// maxDepth levels. maxDepth == 0 means unlimited depth. Cycles are detected
// via the visited set and marked with Cycle: true without further expansion.
func FindCallersRecursive(db *sql.DB, calleeName string, maxDepth int) ([]CallerNode, error) {
	if calleeName == "" {
		return nil, fmt.Errorf("FindCallersRecursive: callee name must not be empty")
	}
	visited := make(map[string]bool)
	visited[calleeName] = true
	return findCallersLevel(db, calleeName, maxDepth, 1, visited)
}

// findCallersLevel is the recursive helper. currentDepth is 1-based.
func findCallersLevel(
	db *sql.DB,
	calleeName string,
	maxDepth, currentDepth int,
	visited map[string]bool,
) ([]CallerNode, error) {
	rows, err := FindCallers(db, calleeName)
	if err != nil {
		return nil, err
	}

	nodes := make([]CallerNode, 0, len(rows))
	for _, r := range rows {
		node := CallerNode{
			CallerFile: r.CallerFile,
			CallerName: r.CallerName,
			CalleeName: r.CalleeName,
			Line:       r.Line,
		}

		// The next level searches for callers of this caller_name.
		// If caller_name is empty (file scope) there is nothing to recurse into.
		nextTarget := r.CallerName
		if nextTarget == "" {
			nodes = append(nodes, node)
			continue
		}

		// Cycle detection.
		if visited[nextTarget] {
			node.Cycle = true
			nodes = append(nodes, node)
			continue
		}

		// Depth limit check.
		if maxDepth > 0 && currentDepth >= maxDepth {
			nodes = append(nodes, node)
			continue
		}

		// Recurse.
		visited[nextTarget] = true
		children, err := findCallersLevel(db, nextTarget, maxDepth, currentDepth+1, visited)
		if err != nil {
			return nil, err
		}
		// Backtrack: allow the same symbol to appear in independent branches.
		delete(visited, nextTarget)

		node.Callers = children
		nodes = append(nodes, node)
	}

	return nodes, nil
}
