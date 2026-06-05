package graph

import (
	"sort"
	"strings"
)

// FilterGraph applies the filtering options encoded in opts to g and returns a
// new Graph containing only the nodes and edges that survive the filter chain.
// The input graph is never modified — a new Graph is always allocated.
//
// Filters are applied in this documented order:
//
//  1. exclude    — Remove every node whose Package field contains the exclude
//     substring (case-insensitive). Edges incident to a removed node are also
//     removed.
//
//  2. focus      — Keep only nodes whose Package field contains the focus
//     substring (case-insensitive). These are the "seed" nodes for the
//     subsequent depth-expansion step. When focus is empty all current nodes
//     are treated as seeds.
//
//  3. depth      — Starting from the seed set, BFS-traverse outward along
//     outgoing edges, keeping every visited node up to depth hops away.
//     Depth 0 means unlimited (no depth filter). When focus is also empty,
//     the BFS starts from every root node (a node with no incoming edges).
//
//  4. min-calls  — Remove edges whose CallCount is strictly below the
//     threshold. Nodes are not removed even when they become isolated.
//
// When every option is at its zero value the input graph is returned unchanged
// (identity filter).
func FilterGraph(g *Graph, opts GraphOptions) *Graph {
	if g == nil {
		return emptyGraph()
	}

	// Fast path — no filtering required.
	if opts.Exclude == "" && opts.Focus == "" && opts.Depth == 0 && opts.MinCalls == 0 {
		return g
	}

	// Defensive copy so we never alias the input slices.
	nodes := append([]Node(nil), g.Nodes...)
	edges := append([]Edge(nil), g.Edges...)

	// -----------------------------------------------------------------------
	// Step 1 — exclude
	// -----------------------------------------------------------------------
	if opts.Exclude != "" {
		excludeLower := strings.ToLower(opts.Exclude)
		kept := nodes[:0]
		keepSet := make(map[string]bool, len(nodes))
		for _, n := range nodes {
			if !strings.Contains(strings.ToLower(n.Package), excludeLower) {
				keepSet[n.ID] = true
				kept = append(kept, n)
			}
		}
		nodes = kept
		edges = keepEdgesBothEnds(edges, keepSet)
	}

	// -----------------------------------------------------------------------
	// Step 2 — focus
	// Step 3 — depth  (combined: seed-set selection + BFS expansion)
	// -----------------------------------------------------------------------
	if opts.Focus != "" || opts.Depth > 0 {
		// Determine the seed set (nodes whose Package matches the focus
		// substring, or the full node set when focus is empty).
		var seedSet map[string]bool
		if opts.Focus != "" {
			focusLower := strings.ToLower(opts.Focus)
			seedSet = make(map[string]bool)
			for _, n := range nodes {
				if strings.Contains(strings.ToLower(n.Package), focusLower) {
					seedSet[n.ID] = true
				}
			}
		}

		// ---- Step 3 — depth (BFS from seeds) ----
		var keepSet map[string]bool
		if opts.Depth > 0 {
			// When focus was empty we start from root nodes.
			if seedSet == nil {
				seedSet = findRootNodes(nodes, edges)
			}
			if len(seedSet) == 0 {
				return emptyGraph()
			}
			keepSet = bfsWithinDepth(nodes, edges, seedSet, opts.Depth)
		} else if opts.Focus != "" {
			// Depth 0 + focus set → keep only the matching seed nodes.
			keepSet = seedSet
		}

		if keepSet != nil {
			kept := nodes[:0]
			for _, n := range nodes {
				if keepSet[n.ID] {
					kept = append(kept, n)
				}
			}
			nodes = kept
			edges = keepEdgesBothEnds(edges, keepSet)
		}
	}

	// -----------------------------------------------------------------------
	// Step 4 — min-calls
	// -----------------------------------------------------------------------
	if opts.MinCalls > 0 {
		filtered := edges[:0]
		for _, e := range edges {
			if e.CallCount >= opts.MinCalls {
				filtered = append(filtered, e)
			}
		}
		edges = filtered
	}

	// Deterministic ordering — required by the "deterministic output" contract.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
	for i := range edges {
		sort.Strings(edges[i].Functions)
	}

	return &Graph{Nodes: nodes, Edges: edges}
}

// keepEdgesBothEnds filters edges to those where both Source and Target are
// present in the nodeSet. Edges with either end missing are dropped.
func keepEdgesBothEnds(edges []Edge, nodeSet map[string]bool) []Edge {
	kept := make([]Edge, 0, len(edges))
	for _, e := range edges {
		if nodeSet[e.Source] && nodeSet[e.Target] {
			kept = append(kept, e)
		}
	}
	return kept
}

// findRootNodes returns the set of node IDs that have no incoming edges in the
// current edge set. A node with no edges at all is also a root.
func findRootNodes(nodes []Node, edges []Edge) map[string]bool {
	hasIncoming := make(map[string]bool, len(nodes))
	for _, e := range edges {
		hasIncoming[e.Target] = true
	}
	roots := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if !hasIncoming[n.ID] {
			roots[n.ID] = true
		}
	}
	return roots
}

// bfsWithinDepth traverses the graph outward from the seed set following
// outgoing edges, up to maxDepth hops. maxDepth 0 is treated as unlimited
// (the entire connected component is returned).
//
// The algorithm uses a simple FIFO queue and a visited set for cycle
// detection. Nodes not reachable from any seed are excluded from the result.
func bfsWithinDepth(nodes []Node, edges []Edge, seeds map[string]bool, maxDepth int) map[string]bool {
	visited := make(map[string]bool, len(nodes))

	// Build adjacency lists (outgoing edges grouped by source).
	adj := make(map[string][]string, len(nodes))
	for _, e := range edges {
		adj[e.Source] = append(adj[e.Source], e.Target)
	}

	type item struct {
		id    string
		depth int
	}

	queue := make([]item, 0, len(seeds))
	for id := range seeds {
		if !visited[id] {
			visited[id] = true
			queue = append(queue, item{id: id, depth: 0})
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if maxDepth > 0 && cur.depth >= maxDepth {
			continue
		}

		for _, child := range adj[cur.id] {
			if !visited[child] {
				visited[child] = true
				queue = append(queue, item{id: child, depth: cur.depth + 1})
			}
		}
	}

	return visited
}
