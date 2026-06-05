package graph

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/Utahjezz/mimir/pkg/workspace"
)

// EnrichWithWorkspaceLinks adds cross-repo edges to g from workspace link
// declarations. Only links involving the current repo (identified by root)
// are used. wsDB is an already-opened workspace database.
//
// Symbols in the current repo are resolved to their package paths via the
// index. Symbols in remote repos are represented as external nodes with the
// other repo's ID. When no matching links exist, g is returned unchanged.
func EnrichWithWorkspaceLinks(g *Graph, wsDB *sql.DB, root string) error {
	if g == nil {
		return nil
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("enrich workspace links: resolve root: %w", err)
	}

	currentRepoID := indexer.RepoID(absRoot)

	links, err := workspace.ListLinks(wsDB, workspace.LinkQuery{})
	if err != nil {
		return fmt.Errorf("enrich workspace links: list links: %w", err)
	}

	// Open the index DB to resolve symbols to packages.
	indexDB, err := indexer.OpenIndex(absRoot)
	if err != nil {
		return fmt.Errorf("enrich workspace links: open index: %w", err)
	}
	defer indexDB.Close()

	// Build a set of existing node IDs for deduplication.
	nodeSet := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeSet[n.ID] = true
	}

	for _, link := range links {
		// Only process links where the current repo appears on either side.
		if link.SrcRepoID != currentRepoID && link.DstRepoID != currentRepoID {
			continue
		}

		var srcPkg, dstPkg string
		srcIsExternal := link.SrcRepoID != currentRepoID
		dstIsExternal := link.DstRepoID != currentRepoID

		// Resolve the symbol that lives in the current repo. The remote
		// symbol cannot be resolved via this index, so we build an
		// external node identifier for the other repo's side.
		if !srcIsExternal {
			srcPkg, err = resolveLinkSymbolPackage(indexDB, absRoot, link.SrcSymbol)
			if err != nil {
				return fmt.Errorf("enrich workspace links: resolve src %q: %w", link.SrcSymbol, err)
			}
		} else {
			srcPkg = externalNodeID(link.SrcRepoID, link.SrcSymbol)
		}
		if !dstIsExternal {
			dstPkg, err = resolveLinkSymbolPackage(indexDB, absRoot, link.DstSymbol)
			if err != nil {
				return fmt.Errorf("enrich workspace links: resolve dst %q: %w", link.DstSymbol, err)
			}
		} else {
			dstPkg = externalNodeID(link.DstRepoID, link.DstSymbol)
		}

		// Skip if either endpoint is empty.
		if srcPkg == "" || dstPkg == "" {
			continue
		}

		// Add external nodes if not already present.
		if srcIsExternal && !nodeSet[srcPkg] {
			g.Nodes = append(g.Nodes, Node{
				ID:      srcPkg,
				Label:   srcPkg,
				Package: srcPkg,
				RepoID:  link.SrcRepoID,
			})
			nodeSet[srcPkg] = true
		}
		if dstIsExternal && !nodeSet[dstPkg] {
			g.Nodes = append(g.Nodes, Node{
				ID:      dstPkg,
				Label:   dstPkg,
				Package: dstPkg,
				RepoID:  link.DstRepoID,
			})
			nodeSet[dstPkg] = true
		}

		// Determine the "other" repo ID for edge metadata — it is the
		// repo that is NOT the current working repo.
		var otherRepoID string
		if srcIsExternal {
			otherRepoID = link.SrcRepoID
		} else {
			otherRepoID = link.DstRepoID
		}

		// Build the function list from the link symbols.
		functions := buildLinkFunctions(link.SrcSymbol, link.DstSymbol)

		g.Edges = append(g.Edges, Edge{
			Source:      srcPkg,
			Target:      dstPkg,
			CallCount:   1,
			Functions:   functions,
			IsCrossRepo: true,
			RepoID:      otherRepoID,
			LinkMeta:    cloneMeta(link.Meta),
		})
	}

	// Restore deterministic ordering after appending new nodes and edges.
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].Source != g.Edges[j].Source {
			return g.Edges[i].Source < g.Edges[j].Source
		}
		return g.Edges[i].Target < g.Edges[j].Target
	})
	for i := range g.Edges {
		sort.Strings(g.Edges[i].Functions)
	}

	return nil
}

// resolveLinkSymbolPackage looks up a symbol by name in the index and returns
// its package path. Returns ("", nil) when the symbol is not found or is
// ambiguous (multiple packages contain a symbol with the same name).
func resolveLinkSymbolPackage(db *sql.DB, root, symbolName string) (string, error) {
	if symbolName == "" {
		return "", nil
	}

	rows, err := indexer.SearchSymbols(db, indexer.SearchQuery{Name: symbolName})
	if err != nil {
		return "", err
	}

	packages := make(map[string]struct{})
	for _, row := range rows {
		if row.Type != indexer.Function && row.Type != indexer.Method {
			continue
		}
		pkg, err := packageFromFile(root, row.FilePath)
		if err != nil {
			return "", err
		}
		packages[pkg] = struct{}{}
	}

	pkg, ok := uniquePackage(packages)
	if !ok {
		return "", nil
	}
	return pkg, nil
}

// externalNodeID creates a stable identifier for a graph node that represents
// a symbol in a different repository. The format is <repoID>/<symbol>.
func externalNodeID(repoID, symbol string) string {
	if symbol == "" {
		return repoID
	}
	return repoID + "/" + symbol
}

// buildLinkFunctions returns a sorted, deduplicated slice of function names
// from the two symbol fields of a workspace link.
func buildLinkFunctions(srcSymbol, dstSymbol string) []string {
	set := make(map[string]struct{}, 2)
	if srcSymbol != "" {
		set[srcSymbol] = struct{}{}
	}
	if dstSymbol != "" {
		set[dstSymbol] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for s := range set {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

// cloneMeta creates a shallow copy of a string map to prevent aliasing.
func cloneMeta(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
