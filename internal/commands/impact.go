package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/Utahjezz/mimir/pkg/indexer"
	workspacepkg "github.com/Utahjezz/mimir/pkg/workspace"
	"github.com/spf13/cobra"
)

var (
	impactSimulateSymbol    string
	impactSimulateChangeRaw string
	impactSimulateMaxDepth  int
	impactSimulateCrossRepo bool
	impactSimulateJSON      bool
	impactSimulateNoRefresh bool
	impactSimulateWorkspace string
)

var impactCmd = &cobra.Command{
	Use:   "impact",
	Short: "Counterfactual impact analysis commands",
	Long: `Estimate the blast radius of hypothetical symbol changes.

Use the simulate subcommand to parse a proposed change and prepare an
impact-simulation input payload for downstream analysis.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var impactSimulateCmd = &cobra.Command{
	Use:   "simulate <root>",
	Short: "Simulate blast radius and risk for a hypothetical symbol change",
	Long: `Simulate the impact of changing a symbol and return a deterministic analysis
of affected callers/paths, risk score, confidence, and planning signals.

--change format:
  kind[:key=value[:key=value...]]

Examples:
  --change "rename_symbol:to=NewName"
  --change "add_required_param:param_name=currency:param_type_to=string"
  --change "param_type_change:param_name=limit:param_type_from=int:param_type_to=int64"

JSON mode emits schema_version=impact-sim/v1 for agent/tool consumption.
Text mode emits a concise human-oriented summary with top impacts and checks.`,
	Args: cobra.ExactArgs(1),
	RunE: runImpactSimulate,
}

func runImpactSimulate(cmd *cobra.Command, args []string) error {
	if impactSimulateSymbol == "" {
		return fmt.Errorf("--symbol is required")
	}
	if impactSimulateChangeRaw == "" {
		return fmt.Errorf("--change is required")
	}
	if impactSimulateMaxDepth < 0 {
		return fmt.Errorf("--max-depth must be >= 0 (got %d)", impactSimulateMaxDepth)
	}

	change, err := parseImpactChangeDescriptor(impactSimulateChangeRaw)
	if err != nil {
		return err
	}

	workspaceName := impactSimulateWorkspace
	if workspaceName == "" {
		workspaceName = "default"
	}

	input := indexer.ImpactSimulationInput{
		Symbol:    impactSimulateSymbol,
		Workspace: workspaceName,
		Repo:      args[0],
		Change:    change,
		Options: &indexer.ImpactOptions{
			MaxDepth:         impactSimulateMaxDepth,
			IncludeCrossRepo: impactSimulateCrossRepo,
		},
	}

	db, err := indexer.OpenIndex(args[0])
	if err != nil {
		return fmt.Errorf("cannot open index: %w", err)
	}
	defer db.Close()

	var refreshed bool
	if !impactSimulateNoRefresh {
		stats, err := indexer.AutoRefresh(args[0], db, RefreshThreshold)
		if err != nil {
			return fmt.Errorf("auto-refresh: %w", err)
		}
		refreshed = stats.Added > 0 || stats.Updated > 0 || stats.Removed > 0 || stats.Unchanged > 0 || stats.Errors > 0
	}

	options := indexer.ImpactPropagationOptions{
		MaxDepth:         impactSimulateMaxDepth,
		IncludeCrossRepo: impactSimulateCrossRepo,
	}
	if impactSimulateCrossRepo {
		repoRoots, links, err := loadImpactWorkspaceContext(input.Workspace, args[0])
		if err != nil {
			return err
		}
		options.RepoRoots = repoRoots
		options.WorkspaceLinks = links
	}

	propagation, err := indexer.PropagateImpact(args[0], input, options)
	if err != nil {
		return err
	}

	scored, aggregate := indexer.ScoreImpactItems(propagation.Impacts, nil)
	planningSignals, recommendedOrder, requiredChecks, notifyOwners, alternatives := indexer.BuildImpactPlanning(
		propagation.Start,
		propagation.Scope,
		scored,
		aggregate,
		propagation.Warnings,
	)

	lastIndexedAt, err := indexer.GetLastIndexedAt(db)
	if err != nil {
		return fmt.Errorf("read last indexed at: %w", err)
	}
	indexAgeSeconds := 0
	if !lastIndexedAt.IsZero() {
		age := time.Since(lastIndexedAt.UTC())
		if age < 0 {
			age = 0
		}
		indexAgeSeconds = int(age.Seconds())
	}

	result := indexer.ImpactSimulationResult{
		SchemaVersion:    indexer.ImpactSchemaVersion,
		RunID:            fmt.Sprintf("impact-%d", time.Now().UTC().UnixNano()),
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		Input:            input,
		Scores:           aggregate,
		Scope:            propagation.Scope,
		Summary:          buildImpactSummary(propagation.Start, aggregate, propagation.Scope),
		Impacts:          scoredItems(scored),
		RecommendedOrder: recommendedOrder,
		RequiredChecks:   requiredChecks,
		NotifyOwners:     notifyOwners,
		Alternatives:     alternatives,
		PlanningSignals:  planningSignals,
		Diagnostics: indexer.ImpactDiagnostics{
			IndexRefreshed:     refreshed,
			IndexAgeSeconds:    indexAgeSeconds,
			WorkspaceLinksUsed: len(options.WorkspaceLinks),
			AnalysisDepth:      impactSimulateMaxDepth,
			Warnings:           propagation.Warnings,
		},
	}
	result.HighRiskBoundaries = buildHighRiskBoundaries(result.Impacts)

	if impactSimulateJSON {
		return writeImpactResultJSON(cmd, result)
	}
	return writeImpactResultHuman(cmd, result)
}

func parseImpactChangeDescriptor(raw string) (indexer.ImpactChange, error) {
	parts := strings.Split(raw, ":")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return indexer.ImpactChange{}, fmt.Errorf("invalid --change %q: expected kind[:key=value[:key=value...]]", raw)
	}

	change := indexer.ImpactChange{Kind: indexer.ImpactChangeKind(strings.TrimSpace(parts[0]))}
	if !isSupportedImpactChangeKind(change.Kind) {
		return indexer.ImpactChange{}, fmt.Errorf("unsupported change kind %q; supported: %s", change.Kind, strings.Join(supportedImpactChangeKinds(), ", "))
	}

	for _, token := range parts[1:] {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		kv := strings.SplitN(token, "=", 2)
		if len(kv) != 2 {
			return indexer.ImpactChange{}, fmt.Errorf("invalid --change token %q: expected key=value", token)
		}

		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key == "" || val == "" {
			return indexer.ImpactChange{}, fmt.Errorf("invalid --change token %q: key and value must be non-empty", token)
		}

		switch key {
		case "from":
			change.From = val
		case "to":
			change.To = val
		case "param", "param_name":
			change.ParamName = val
		case "param_type_from":
			change.ParamTypeFrom = val
		case "param_type_to", "param_type":
			change.ParamTypeTo = val
		case "return_type_from":
			change.ReturnTypeFrom = val
		case "return_type_to", "return_type":
			change.ReturnTypeTo = val
		case "notes":
			change.Notes = val
		default:
			return indexer.ImpactChange{}, fmt.Errorf("unsupported --change key %q; allowed keys: from,to,param_name,param_type_from,param_type_to,return_type_from,return_type_to,notes", key)
		}
	}

	if err := validateImpactChange(change); err != nil {
		return indexer.ImpactChange{}, err
	}

	return change, nil
}

func validateImpactChange(change indexer.ImpactChange) error {
	switch change.Kind {
	case indexer.ImpactChangeRenameSymbol:
		if change.To == "" {
			return fmt.Errorf("invalid rename_symbol change: missing to=<new_symbol_name>")
		}
	case indexer.ImpactChangeAddRequiredParam, indexer.ImpactChangeAddOptionalParam, indexer.ImpactChangeRemoveParam:
		if change.ParamName == "" {
			return fmt.Errorf("invalid %s change: missing param_name=<name>", change.Kind)
		}
	case indexer.ImpactChangeParamTypeChange:
		if change.ParamName == "" || change.ParamTypeTo == "" {
			return fmt.Errorf("invalid param_type_change: requires param_name=<name> and param_type_to=<type>")
		}
	case indexer.ImpactChangeReturnTypeChange:
		if change.ReturnTypeTo == "" {
			return fmt.Errorf("invalid return_type_change: missing return_type_to=<type>")
		}
	case indexer.ImpactChangeVisibilityChange, indexer.ImpactChangeExportChange:
		if change.To == "" && change.From == "" {
			return fmt.Errorf("invalid %s change: provide from=<old> and/or to=<new>", change.Kind)
		}
	case indexer.ImpactChangeBehaviorHint:
		if change.Notes == "" {
			return fmt.Errorf("invalid behavior_change_hint: missing notes=<description>")
		}
	}
	return nil
}

func isSupportedImpactChangeKind(kind indexer.ImpactChangeKind) bool {
	for _, candidate := range supportedImpactChangeKinds() {
		if candidate == string(kind) {
			return true
		}
	}
	return false
}

func supportedImpactChangeKinds() []string {
	return []string{
		string(indexer.ImpactChangeRenameSymbol),
		string(indexer.ImpactChangeAddRequiredParam),
		string(indexer.ImpactChangeAddOptionalParam),
		string(indexer.ImpactChangeRemoveParam),
		string(indexer.ImpactChangeParamTypeChange),
		string(indexer.ImpactChangeReturnTypeChange),
		string(indexer.ImpactChangeVisibilityChange),
		string(indexer.ImpactChangeExportChange),
		string(indexer.ImpactChangeBehaviorHint),
	}
}

func loadImpactWorkspaceContext(workspaceName, root string) (map[string]string, []indexer.ImpactWorkspaceLink, error) {
	name := workspaceName
	if name == "" || name == "default" {
		if active, err := workspacepkg.GetCurrentWorkspace(); err == nil {
			name = active
		}
	}

	wsDB, err := workspacepkg.OpenWorkspace(name)
	if err != nil {
		return nil, nil, nil
	}
	defer wsDB.Close()

	repos, err := workspacepkg.ListRepositories(wsDB)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot list workspace repositories: %w", err)
	}

	repoRoots := map[string]string{}
	for _, repo := range repos {
		repoRoots[repo.ID] = repo.Path
	}
	repoRoots[indexer.RepoID(root)] = root

	links, err := workspacepkg.ListLinks(wsDB, workspacepkg.LinkQuery{})
	if err != nil {
		return nil, nil, fmt.Errorf("cannot list workspace links: %w", err)
	}

	out := make([]indexer.ImpactWorkspaceLink, 0, len(links))
	for _, link := range links {
		out = append(out, indexer.ImpactWorkspaceLink{
			ID:        link.ID,
			SrcRepoID: link.SrcRepoID,
			SrcSymbol: link.SrcSymbol,
			SrcFile:   link.SrcFile,
			DstRepoID: link.DstRepoID,
			DstSymbol: link.DstSymbol,
			DstFile:   link.DstFile,
			Note:      link.Note,
		})
	}

	return repoRoots, out, nil
}
