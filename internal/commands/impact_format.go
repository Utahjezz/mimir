package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/spf13/cobra"
)

func writeImpactResultJSON(cmd *cobra.Command, result indexer.ImpactSimulationResult) error {
	ensureImpactResultSlices(&result)
	return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
}

func writeImpactResultHuman(cmd *cobra.Command, result indexer.ImpactSimulationResult) error {
	w := cmd.OutOrStdout()

	fmt.Fprintln(w, result.Summary.Headline)
	for _, bullet := range result.Summary.Bullets {
		fmt.Fprintf(w, "  - %s\n", bullet)
	}

	fmt.Fprintf(w, "\nRisk: %d (%s)  Confidence: %.2f\n", result.Scores.RiskScore, result.Scores.RiskTier, result.Scores.Confidence)
	fmt.Fprintf(w, "Scope: direct=%d indirect=%d symbols=%d files=%d repos=%d cross_edges=%d\n",
		result.Scope.DirectCallers,
		result.Scope.IndirectCallers,
		result.Scope.AffectedSymbols,
		result.Scope.AffectedFiles,
		result.Scope.AffectedRepos,
		result.Scope.CrossBoundaryEdges,
	)

	if len(result.Impacts) == 0 {
		fmt.Fprintln(w, "\nNo impacted symbols predicted.")
	} else {
		fmt.Fprintln(w, "\nTop impacts:")
		for i, impact := range topNImpacts(result.Impacts, 5) {
			fmt.Fprintf(w, "  %d) %s | risk=%d(%s) conf=%.2f dist=%d\n",
				i+1,
				formatImpactTarget(impact.Target),
				impact.RiskScore,
				impact.RiskTier,
				impact.Confidence,
				impact.Distance,
			)
			if len(impact.ReasonCodes) > 0 {
				fmt.Fprintf(w, "     reasons: %s\n", strings.Join(reasonCodesToStrings(impact.ReasonCodes), ", "))
			}
		}
	}

	if len(result.RequiredChecks) > 0 {
		fmt.Fprintln(w, "\nRequired checks:")
		for _, check := range result.RequiredChecks {
			blocking := "non-blocking"
			if check.Blocking {
				blocking = "blocking"
			}
			fmt.Fprintf(w, "  - [%s] %s (%s) target=%s\n", check.Priority, check.Type, blocking, check.Target)
		}
	}

	if len(result.RecommendedOrder) > 0 {
		fmt.Fprintln(w, "\nRecommended order:")
		for _, step := range result.RecommendedOrder {
			fmt.Fprintf(w, "  %d. %s\n", step.Step, step.Action)
		}
	}

	if len(result.NotifyOwners) > 0 {
		fmt.Fprintln(w, "\nNotify owners:")
		for _, owner := range result.NotifyOwners {
			fmt.Fprintf(w, "  - %s (%s): %s\n", owner.Owner.ID, owner.Owner.Type, owner.Reason)
		}
	}

	if len(result.Diagnostics.Warnings) > 0 {
		fmt.Fprintln(w, "\nWarnings:")
		for _, warning := range result.Diagnostics.Warnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}

	return nil
}

func buildImpactSummary(start indexer.ImpactSymbolRef, scores indexer.ImpactScores, scope indexer.ImpactScope) indexer.ImpactSummary {
	headline := fmt.Sprintf("Impact simulation for %s", start.Symbol)
	bullets := []string{
		fmt.Sprintf("Risk score %d (%s), confidence %.2f", scores.RiskScore, scores.RiskTier, scores.Confidence),
		fmt.Sprintf("Predicted impacts: %d symbols across %d files in %d repos", scope.AffectedSymbols, scope.AffectedFiles, scope.AffectedRepos),
	}
	if scope.CrossBoundaryEdges > 0 {
		bullets = append(bullets, fmt.Sprintf("Cross-boundary edges detected: %d", scope.CrossBoundaryEdges))
	}
	return indexer.ImpactSummary{Headline: headline, Bullets: bullets}
}

func scoredItems(scored []indexer.ImpactScoreResult) []indexer.ImpactItem {
	items := make([]indexer.ImpactItem, 0, len(scored))
	for _, entry := range scored {
		items = append(items, entry.Item)
	}
	return items
}

func buildHighRiskBoundaries(impacts []indexer.ImpactItem) []indexer.ImpactBoundary {
	boundaries := make([]indexer.ImpactBoundary, 0)
	for _, impact := range impacts {
		if impact.RiskScore < 65 {
			continue
		}
		if !containsImpactReason(impact.ReasonCodes, indexer.ImpactReasonCrossRepoContractEdge) {
			continue
		}
		boundaries = append(boundaries, indexer.ImpactBoundary{
			Kind:        indexer.ImpactBoundarySharedType,
			SourceRepo:  "unknown",
			TargetRepo:  impact.Target.Repo,
			RiskScore:   impact.RiskScore,
			ReasonCodes: []indexer.ImpactReasonCode{indexer.ImpactReasonCrossRepoContractEdge},
		})
	}
	return boundaries
}

func containsImpactReason(reasons []indexer.ImpactReasonCode, target indexer.ImpactReasonCode) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}

func ensureImpactResultSlices(result *indexer.ImpactSimulationResult) {
	if result.Impacts == nil {
		result.Impacts = []indexer.ImpactItem{}
	}
	if result.HighRiskBoundaries == nil {
		result.HighRiskBoundaries = []indexer.ImpactBoundary{}
	}
	if result.RecommendedOrder == nil {
		result.RecommendedOrder = []indexer.ImpactRecommendedStep{}
	}
	if result.RequiredChecks == nil {
		result.RequiredChecks = []indexer.ImpactRequiredCheck{}
	}
	if result.NotifyOwners == nil {
		result.NotifyOwners = []indexer.ImpactOwnerNotification{}
	}
	if result.Alternatives == nil {
		result.Alternatives = []indexer.ImpactAlternativePlan{}
	}
	if result.Diagnostics.Warnings == nil {
		result.Diagnostics.Warnings = []string{}
	}
	if result.Summary.Bullets == nil {
		result.Summary.Bullets = []string{}
	}
	if result.PlanningSignals.AgentHints == nil {
		result.PlanningSignals.AgentHints = []indexer.ImpactAgentHint{}
	}
}

func topNImpacts(items []indexer.ImpactItem, n int) []indexer.ImpactItem {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func formatImpactTarget(target indexer.ImpactSymbolRef) string {
	if target.File == "" {
		return fmt.Sprintf("%s@%s", target.Symbol, target.Repo)
	}
	return fmt.Sprintf("%s (%s:%d)@%s", target.Symbol, target.File, target.Line, target.Repo)
}

func reasonCodesToStrings(codes []indexer.ImpactReasonCode) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		out = append(out, string(code))
	}
	return out
}
