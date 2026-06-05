package graph

import (
	"database/sql"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Utahjezz/mimir/pkg/indexer"
)

// BuildGraph aggregates call refs into a graph at the requested scope.
func BuildGraph(opts BuildOptions) (*Graph, error) {
	if len(opts.Refs) == 0 {
		return emptyGraph(), nil
	}

	if _, err := normalizeScope(opts.Options.Scope); err != nil {
		return nil, fmt.Errorf("build graph: %w", err)
	}
	if opts.Root == "" {
		return nil, fmt.Errorf("build graph: root is required")
	}
	if opts.IndexDB == nil {
		return nil, fmt.Errorf("build graph: index db is required when refs are provided")
	}

	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("build graph: resolve root: %w", err)
	}

	builder := newPackageBuilder(absRoot, opts.IndexDB)
	for _, ref := range opts.Refs {
		if err := builder.addRef(ref); err != nil {
			return nil, fmt.Errorf("build graph: %w", err)
		}
	}

	return builder.graph(), nil
}

type packageBuilder struct {
	root        string
	db          *sql.DB
	nodes       map[string]*Node
	edges       map[edgeKey]*edgeAccumulator
	calleeCache map[string]calleeResolution
}

type edgeKey struct {
	source string
	target string
}

type edgeAccumulator struct {
	edge        Edge
	functionSet map[string]struct{}
}

type calleeResolution struct {
	pkg string
	ok  bool
}

func newPackageBuilder(root string, db *sql.DB) *packageBuilder {
	return &packageBuilder{
		root:        root,
		db:          db,
		nodes:       make(map[string]*Node),
		edges:       make(map[edgeKey]*edgeAccumulator),
		calleeCache: make(map[string]calleeResolution),
	}
}

func (b *packageBuilder) addRef(ref indexer.RefRow) error {
	callerPackage, err := packageFromFile(b.root, ref.CallerFile)
	if err != nil {
		return fmt.Errorf("resolve caller package for %q: %w", ref.CallerFile, err)
	}

	calleePackage, ok, err := b.resolveCalleePackage(ref.CalleeName)
	if err != nil {
		return fmt.Errorf("resolve callee package for %q: %w", ref.CalleeName, err)
	}
	if !ok {
		return nil
	}

	b.ensureNode(callerPackage)
	b.ensureNode(calleePackage)
	if callerPackage == calleePackage {
		b.nodes[callerPackage].CallCount++
		return nil
	}

	b.nodes[callerPackage].CallCount++
	b.nodes[calleePackage].CallCount++

	edge := b.ensureEdge(callerPackage, calleePackage)
	edge.edge.CallCount++
	edge.addFunction(ref.CallerName)
	edge.addFunction(ref.CalleeName)

	return nil
}

func (b *packageBuilder) resolveCalleePackage(calleeName string) (string, bool, error) {
	if calleeName == "" {
		return "", false, nil
	}
	if cached, ok := b.calleeCache[calleeName]; ok {
		return cached.pkg, cached.ok, nil
	}

	rows, err := indexer.SearchSymbols(b.db, indexer.SearchQuery{Name: calleeName})
	if err != nil {
		return "", false, err
	}

	packages := make(map[string]struct{})
	for _, row := range rows {
		if row.Type != indexer.Function && row.Type != indexer.Method {
			continue
		}

		pkg, err := packageFromFile(b.root, row.FilePath)
		if err != nil {
			return "", false, err
		}
		packages[pkg] = struct{}{}
	}

	pkg, ok := uniquePackage(packages)
	b.calleeCache[calleeName] = calleeResolution{pkg: pkg, ok: ok}
	return pkg, ok, nil
}

func (b *packageBuilder) ensureNode(pkg string) {
	if _, ok := b.nodes[pkg]; ok {
		return
	}

	b.nodes[pkg] = &Node{
		ID:      pkg,
		Label:   pkg,
		Package: pkg,
	}
}

func (b *packageBuilder) ensureEdge(source, target string) *edgeAccumulator {
	key := edgeKey{source: source, target: target}
	if edge, ok := b.edges[key]; ok {
		return edge
	}

	edge := &edgeAccumulator{
		edge: Edge{
			Source:    source,
			Target:    target,
			Functions: []string{},
		},
		functionSet: make(map[string]struct{}),
	}
	b.edges[key] = edge
	return edge
}

func (b *packageBuilder) graph() *Graph {
	nodes := sortedNodes(b.nodes)
	edges := sortedEdges(b.edges)
	return &Graph{Nodes: nodes, Edges: edges}
}

func (e *edgeAccumulator) addFunction(name string) {
	if name == "" {
		return
	}
	if _, ok := e.functionSet[name]; ok {
		return
	}

	e.functionSet[name] = struct{}{}
	e.edge.Functions = append(e.edge.Functions, name)
}

func sortedNodes(nodes map[string]*Node) []Node {
	keys := make([]string, 0, len(nodes))
	for key := range nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]Node, 0, len(keys))
	for _, key := range keys {
		result = append(result, *nodes[key])
	}
	return result
}

func sortedEdges(edges map[edgeKey]*edgeAccumulator) []Edge {
	keys := make([]edgeKey, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		return keys[i].target < keys[j].target
	})

	result := make([]Edge, 0, len(keys))
	for _, key := range keys {
		edge := edges[key].edge
		sort.Strings(edge.Functions)
		result = append(result, edge)
	}
	return result
}

func emptyGraph() *Graph {
	return &Graph{
		Nodes: []Node{},
		Edges: []Edge{},
	}
}

func normalizeScope(scope Scope) (Scope, error) {
	if scope == "" {
		return ScopePackage, nil
	}
	if scope != ScopePackage {
		return "", fmt.Errorf("unsupported graph scope %q", scope)
	}
	return scope, nil
}

func uniquePackage(packages map[string]struct{}) (string, bool) {
	if len(packages) != 1 {
		return "", false
	}
	for pkg := range packages {
		return pkg, true
	}
	return "", false
}

func packageFromFile(root, filePath string) (string, error) {
	rel, err := relativeToRoot(root, filePath)
	if err != nil {
		return "", err
	}

	dir := path.Dir(filepath.ToSlash(rel))
	if dir == "" {
		return ".", nil
	}
	return dir, nil
}

func relativeToRoot(root, filePath string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("root is required")
	}
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}

	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		rel, err := filepath.Rel(root, cleanPath)
		if err != nil {
			return "", fmt.Errorf("make relative path: %w", err)
		}
		cleanPath = rel
	}

	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root %q", filePath, root)
	}

	return cleanPath, nil
}
