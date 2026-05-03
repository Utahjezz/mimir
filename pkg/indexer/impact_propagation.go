package indexer

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// impact_propagation.go — graph propagation for counterfactual impact analysis.

// ImpactWorkspaceLink models a cross-repo symbol edge consumed by propagation.
// It is intentionally decoupled from pkg/workspace to avoid package cycles.
type ImpactWorkspaceLink struct {
	ID        int64  `json:"id"`
	SrcRepoID string `json:"src_repo_id"`
	SrcSymbol string `json:"src_symbol"`
	SrcFile   string `json:"src_file,omitempty"`
	DstRepoID string `json:"dst_repo_id"`
	DstSymbol string `json:"dst_symbol"`
	DstFile   string `json:"dst_file,omitempty"`
	Note      string `json:"note,omitempty"`
}

// ImpactPropagationOptions configures traversal scope and external graph inputs.
type ImpactPropagationOptions struct {
	MaxDepth         int                          `json:"max_depth,omitempty"`
	IncludeCrossRepo bool                         `json:"include_cross_repo,omitempty"`
	RepoRoots        map[string]string            `json:"repo_roots,omitempty"`
	WorkspaceLinks   []ImpactWorkspaceLink        `json:"workspace_links,omitempty"`
	Metadata         map[string]map[string]string `json:"metadata,omitempty"`
}

// ImpactPropagationResult contains propagated impact entries and scope metrics.
type ImpactPropagationResult struct {
	Start    ImpactSymbolRef `json:"start"`
	Impacts  []ImpactItem    `json:"impacts"`
	Scope    ImpactScope     `json:"scope"`
	Warnings []string        `json:"warnings,omitempty"`
}

type impactExpansion struct {
	target      ImpactSymbolRef
	reason      ImpactReasonCode
	breakType   ImpactBreakType
	edge        ImpactEvidence
	traversable bool
	callerHop   bool
	crossRepo   bool
}

type impactQueueNode struct {
	ref      ImpactSymbolRef
	distance int
	path     []ImpactEvidence
}

type impactContext struct {
	primaryRepoID string
	dbByRepo      map[string]*sql.DB
	repoRoots     map[string]string
	symbolCache   map[string][]SymbolRow
	warnings      []string
}

// PropagateImpact traverses callers, refs, and optional cross-repo links from
// the requested symbol and returns deterministic impact entries.
func PropagateImpact(root string, input ImpactSimulationInput, opts ImpactPropagationOptions) (ImpactPropagationResult, error) {
	if strings.TrimSpace(root) == "" {
		return ImpactPropagationResult{}, fmt.Errorf("PropagateImpact: root must not be empty")
	}
	if strings.TrimSpace(input.Symbol) == "" {
		return ImpactPropagationResult{}, fmt.Errorf("PropagateImpact: symbol must not be empty")
	}

	primaryDB, err := OpenIndex(root)
	if err != nil {
		return ImpactPropagationResult{}, fmt.Errorf("PropagateImpact open index: %w", err)
	}

	ctx := newImpactContext(root, primaryDB, opts)
	defer ctx.close()

	start, err := ctx.resolveStartSymbol(input.Symbol)
	if err != nil {
		return ImpactPropagationResult{}, err
	}

	result, err := propagateFromStart(ctx, input, opts, start)
	if err != nil {
		return ImpactPropagationResult{}, err
	}
	return result, nil
}

func newImpactContext(root string, primaryDB *sql.DB, opts ImpactPropagationOptions) *impactContext {
	primaryRepoID := RepoID(root)
	repoRoots := make(map[string]string, len(opts.RepoRoots)+1)
	for repoID, repoRoot := range opts.RepoRoots {
		repoRoots[repoID] = repoRoot
	}
	repoRoots[primaryRepoID] = root

	return &impactContext{
		primaryRepoID: primaryRepoID,
		dbByRepo: map[string]*sql.DB{
			primaryRepoID: primaryDB,
		},
		repoRoots:   repoRoots,
		symbolCache: map[string][]SymbolRow{},
	}
}

func (c *impactContext) close() {
	for _, db := range c.dbByRepo {
		_ = db.Close()
	}
}

func (c *impactContext) addWarning(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

func (c *impactContext) resolveStartSymbol(symbol string) (ImpactSymbolRef, error) {
	rows, err := c.searchSymbolRows(c.primaryRepoID, symbol)
	if err != nil {
		return ImpactSymbolRef{}, err
	}
	if len(rows) == 0 {
		return ImpactSymbolRef{}, fmt.Errorf("PropagateImpact: symbol %q not found in root repository", symbol)
	}
	r := rows[0]
	return ImpactSymbolRef{Symbol: r.Name, File: r.FilePath, Repo: c.primaryRepoID, Line: r.StartLine}, nil
}

func (c *impactContext) dbForRepo(repoID string) (*sql.DB, error) {
	if db, ok := c.dbByRepo[repoID]; ok {
		return db, nil
	}
	repoRoot, ok := c.repoRoots[repoID]
	if !ok {
		return nil, fmt.Errorf("missing repo root for repo_id %q", repoID)
	}
	db, err := OpenIndex(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open index for repo_id %q: %w", repoID, err)
	}
	c.dbByRepo[repoID] = db
	return db, nil
}

func (c *impactContext) searchSymbolRows(repoID, symbol string) ([]SymbolRow, error) {
	cacheKey := repoID + "|" + symbol
	if rows, ok := c.symbolCache[cacheKey]; ok {
		return rows, nil
	}

	db, err := c.dbForRepo(repoID)
	if err != nil {
		return nil, err
	}
	rows, err := SearchSymbols(db, SearchQuery{Name: symbol})
	if err != nil {
		return nil, fmt.Errorf("search symbols for %q in %q: %w", symbol, repoID, err)
	}
	c.symbolCache[cacheKey] = rows
	return rows, nil
}

func (c *impactContext) resolveTargetRefs(repoID, symbol, fileHint string, line int) ([]ImpactSymbolRef, error) {
	rows, err := c.searchSymbolRows(repoID, symbol)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []ImpactSymbolRef{{
			Symbol: symbol,
			File:   fileHint,
			Repo:   repoID,
			Line:   line,
		}}, nil
	}

	refs := make([]ImpactSymbolRef, 0, len(rows))
	for _, r := range rows {
		refs = append(refs, ImpactSymbolRef{
			Symbol: r.Name,
			File:   r.FilePath,
			Repo:   repoID,
			Line:   r.StartLine,
		})
	}
	return refs, nil
}

func propagateFromStart(c *impactContext, input ImpactSimulationInput, opts ImpactPropagationOptions, start ImpactSymbolRef) (ImpactPropagationResult, error) {
	maxDepth := opts.MaxDepth
	if maxDepth < 0 {
		maxDepth = 0
	}

	queue := []impactQueueNode{{ref: start, distance: 0, path: nil}}
	visited := map[string]int{visitedKey(start.Repo, start.Symbol): 0}
	impactByKey := map[string]*ImpactItem{}
	directCallers := map[string]struct{}{}
	indirectCallers := map[string]struct{}{}
	crossBoundaryEdges := map[string]struct{}{}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if maxDepth > 0 && node.distance >= maxDepth {
			continue
		}

		expansions, err := collectExpansions(c, node, opts, input)
		if err != nil {
			return ImpactPropagationResult{}, err
		}

		for _, exp := range expansions {
			nextDistance := node.distance + 1
			nextPath := appendEvidence(node.path, exp.edge)

			mergeImpact(impactByKey, exp, nextDistance, nextPath)
			trackScopeSignals(exp, nextDistance, directCallers, indirectCallers, crossBoundaryEdges)

			if !exp.traversable {
				continue
			}
			nextVisitKey := visitedKey(exp.target.Repo, exp.target.Symbol)
			prevDistance, seen := visited[nextVisitKey]
			if seen && prevDistance <= nextDistance {
				continue
			}
			visited[nextVisitKey] = nextDistance
			queue = append(queue, impactQueueNode{ref: exp.target, distance: nextDistance, path: nextPath})
		}
	}

	impacts := flattenImpactMap(impactByKey)
	scope := buildImpactScope(impacts, directCallers, indirectCallers, crossBoundaryEdges)

	return ImpactPropagationResult{
		Start:    start,
		Impacts:  impacts,
		Scope:    scope,
		Warnings: append([]string(nil), c.warnings...),
	}, nil
}

func collectExpansions(c *impactContext, node impactQueueNode, opts ImpactPropagationOptions, input ImpactSimulationInput) ([]impactExpansion, error) {
	expansions := make([]impactExpansion, 0)

	callers, err := collectCallerExpansions(c, node)
	if err != nil {
		return nil, err
	}
	expansions = append(expansions, callers...)

	refs, err := collectRefExpansions(c, node)
	if err != nil {
		return nil, err
	}
	expansions = append(expansions, refs...)

	if opts.IncludeCrossRepo || shouldUseCrossRepo(input, opts) {
		links, err := collectWorkspaceExpansions(c, node, opts)
		if err != nil {
			return nil, err
		}
		expansions = append(expansions, links...)
	}

	sort.Slice(expansions, func(i, j int) bool {
		a, b := expansions[i], expansions[j]
		if a.target.Repo != b.target.Repo {
			return a.target.Repo < b.target.Repo
		}
		if a.target.File != b.target.File {
			return a.target.File < b.target.File
		}
		if a.target.Symbol != b.target.Symbol {
			return a.target.Symbol < b.target.Symbol
		}
		if a.target.Line != b.target.Line {
			return a.target.Line < b.target.Line
		}
		return string(a.reason) < string(b.reason)
	})

	return expansions, nil
}

func collectCallerExpansions(c *impactContext, node impactQueueNode) ([]impactExpansion, error) {
	db, err := c.dbForRepo(node.ref.Repo)
	if err != nil {
		c.addWarning("caller expansion skipped for repo %s: %v", node.ref.Repo, err)
		return nil, nil
	}
	rows, err := FindCallers(db, node.ref.Symbol)
	if err != nil {
		return nil, fmt.Errorf("collect callers for %s/%s: %w", node.ref.Repo, node.ref.Symbol, err)
	}

	expansions := make([]impactExpansion, 0, len(rows))
	for _, row := range rows {
		targetSymbol := row.CallerName
		traversable := true
		if targetSymbol == "" {
			targetSymbol = "<file scope>"
			traversable = false
		}

		target := ImpactSymbolRef{Symbol: targetSymbol, File: row.CallerFile, Repo: node.ref.Repo, Line: row.Line}
		reason := ImpactReasonIndirectCallChainPropagation
		if node.distance == 0 {
			reason = ImpactReasonDirectCallerSignatureMismatch
		}

		edge := ImpactEvidence{
			Type: ImpactEdgeCall,
			From: node.ref,
			To:   target,
			Meta: map[string]any{
				"relation": "upstream_caller",
				"line":     row.Line,
			},
		}

		expansions = append(expansions, impactExpansion{
			target:      target,
			reason:      reason,
			breakType:   ImpactBreakCompileError,
			edge:        edge,
			traversable: traversable,
			callerHop:   true,
		})
	}

	return expansions, nil
}

func collectRefExpansions(c *impactContext, node impactQueueNode) ([]impactExpansion, error) {
	db, err := c.dbForRepo(node.ref.Repo)
	if err != nil {
		c.addWarning("ref expansion skipped for repo %s: %v", node.ref.Repo, err)
		return nil, nil
	}
	rows, err := SearchRefs(db, RefQuery{CallerName: node.ref.Symbol})
	if err != nil {
		return nil, fmt.Errorf("collect refs for %s/%s: %w", node.ref.Repo, node.ref.Symbol, err)
	}

	expansions := make([]impactExpansion, 0, len(rows))
	for _, row := range rows {
		targets, err := c.resolveTargetRefs(node.ref.Repo, row.CalleeName, row.CallerFile, row.Line)
		if err != nil {
			return nil, err
		}
		for _, target := range targets {
			edge := ImpactEvidence{
				Type: ImpactEdgeCall,
				From: node.ref,
				To:   target,
				Meta: map[string]any{
					"relation": "downstream_callee",
					"line":     row.Line,
				},
			}
			expansions = append(expansions, impactExpansion{
				target:      target,
				reason:      ImpactReasonIndirectCallChainPropagation,
				breakType:   ImpactBreakBehaviorRegression,
				edge:        edge,
				traversable: target.Symbol != "",
			})
		}
	}

	return expansions, nil
}

func collectWorkspaceExpansions(c *impactContext, node impactQueueNode, opts ImpactPropagationOptions) ([]impactExpansion, error) {
	links := append([]ImpactWorkspaceLink(nil), opts.WorkspaceLinks...)
	sort.Slice(links, func(i, j int) bool {
		if links[i].ID != links[j].ID {
			return links[i].ID < links[j].ID
		}
		if links[i].SrcRepoID != links[j].SrcRepoID {
			return links[i].SrcRepoID < links[j].SrcRepoID
		}
		if links[i].SrcSymbol != links[j].SrcSymbol {
			return links[i].SrcSymbol < links[j].SrcSymbol
		}
		if links[i].DstRepoID != links[j].DstRepoID {
			return links[i].DstRepoID < links[j].DstRepoID
		}
		return links[i].DstSymbol < links[j].DstSymbol
	})

	expansions := make([]impactExpansion, 0)
	for _, link := range links {
		if link.SrcRepoID != node.ref.Repo || link.SrcSymbol != node.ref.Symbol {
			continue
		}

		targets, err := c.resolveTargetRefs(link.DstRepoID, link.DstSymbol, link.DstFile, 0)
		if err != nil {
			c.addWarning("workspace link %d skipped: %v", link.ID, err)
			continue
		}

		for _, target := range targets {
			edge := ImpactEvidence{
				Type: ImpactEdgeWorkspaceLink,
				From: node.ref,
				To:   target,
				Meta: map[string]any{
					"link_id": link.ID,
					"note":    link.Note,
				},
			}
			expansions = append(expansions, impactExpansion{
				target:      target,
				reason:      ImpactReasonCrossRepoContractEdge,
				breakType:   ImpactBreakContractBreak,
				edge:        edge,
				traversable: target.Symbol != "",
				crossRepo:   true,
			})
		}
	}

	return expansions, nil
}

func shouldUseCrossRepo(input ImpactSimulationInput, opts ImpactPropagationOptions) bool {
	if opts.IncludeCrossRepo {
		return true
	}
	if input.Options == nil {
		return false
	}
	return input.Options.IncludeCrossRepo
}

func appendEvidence(path []ImpactEvidence, edge ImpactEvidence) []ImpactEvidence {
	next := make([]ImpactEvidence, 0, len(path)+1)
	next = append(next, path...)
	next = append(next, edge)
	return next
}

func mergeImpact(impactByKey map[string]*ImpactItem, exp impactExpansion, distance int, path []ImpactEvidence) {
	key := impactKey(exp.target)
	item, exists := impactByKey[key]
	if !exists {
		impactByKey[key] = &ImpactItem{
			ID:               key,
			Target:           exp.target,
			Distance:         distance,
			RiskScore:        0,
			Confidence:       0,
			RiskTier:         ImpactRiskLow,
			Likelihood:       ImpactLikelihoodPossible,
			BreakTypes:       []ImpactBreakType{exp.breakType},
			ReasonCodes:      []ImpactReasonCode{exp.reason},
			Evidence:         path,
			SuggestedActions: nil,
		}
		return
	}

	if distance < item.Distance {
		item.Distance = distance
		item.Evidence = path
	}

	item.BreakTypes = appendUniqueBreakType(item.BreakTypes, exp.breakType)
	item.ReasonCodes = appendUniqueReason(item.ReasonCodes, exp.reason)
	sort.Slice(item.BreakTypes, func(i, j int) bool { return string(item.BreakTypes[i]) < string(item.BreakTypes[j]) })
	sort.Slice(item.ReasonCodes, func(i, j int) bool { return string(item.ReasonCodes[i]) < string(item.ReasonCodes[j]) })
}

func appendUniqueBreakType(items []ImpactBreakType, value ImpactBreakType) []ImpactBreakType {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func appendUniqueReason(items []ImpactReasonCode, value ImpactReasonCode) []ImpactReasonCode {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func trackScopeSignals(exp impactExpansion, distance int, direct, indirect, cross map[string]struct{}) {
	targetKey := impactKey(exp.target)
	if exp.callerHop {
		if distance == 1 {
			direct[targetKey] = struct{}{}
		} else {
			indirect[targetKey] = struct{}{}
		}
	}
	if exp.crossRepo {
		cross[edgeKey(exp.edge)] = struct{}{}
	}
}

func flattenImpactMap(impactByKey map[string]*ImpactItem) []ImpactItem {
	items := make([]ImpactItem, 0, len(impactByKey))
	for _, item := range impactByKey {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		if a.Target.Repo != b.Target.Repo {
			return a.Target.Repo < b.Target.Repo
		}
		if a.Target.File != b.Target.File {
			return a.Target.File < b.Target.File
		}
		if a.Target.Symbol != b.Target.Symbol {
			return a.Target.Symbol < b.Target.Symbol
		}
		return a.Target.Line < b.Target.Line
	})
	return items
}

func buildImpactScope(impacts []ImpactItem, direct, indirect, cross map[string]struct{}) ImpactScope {
	files := map[string]struct{}{}
	repos := map[string]struct{}{}
	for _, item := range impacts {
		if item.Target.File != "" {
			files[item.Target.File] = struct{}{}
		}
		if item.Target.Repo != "" {
			repos[item.Target.Repo] = struct{}{}
		}
	}

	return ImpactScope{
		DirectCallers:      len(direct),
		IndirectCallers:    len(indirect),
		AffectedSymbols:    len(impacts),
		AffectedFiles:      len(files),
		AffectedRepos:      len(repos),
		CrossBoundaryEdges: len(cross),
	}
}

func visitedKey(repoID, symbol string) string {
	return repoID + "|" + symbol
}

func impactKey(ref ImpactSymbolRef) string {
	return ref.Repo + "|" + ref.File + "|" + ref.Symbol
}

func edgeKey(edge ImpactEvidence) string {
	return string(edge.Type) + "|" + impactKey(edge.From) + "->" + impactKey(edge.To)
}
