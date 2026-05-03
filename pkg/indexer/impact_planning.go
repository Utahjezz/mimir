package indexer

import (
	"fmt"
	"sort"
)

const (
	planningHighRiskThreshold        = 65
	planningCriticalRiskThreshold    = 85
	planningLowConfidenceThreshold   = 0.55
	planningSplitDirectCallerMinimum = 3
	planningMaxBatchCount            = 5
)

// BuildImpactPlanning derives planner-oriented signals and execution guidance
// from propagated/scored impacts using explicit, deterministic rules.
func BuildImpactPlanning(start ImpactSymbolRef, scope ImpactScope, scored []ImpactScoreResult, aggregate ImpactScores, warnings []string) (
	ImpactPlanningSignals,
	[]ImpactRecommendedStep,
	[]ImpactRequiredCheck,
	[]ImpactOwnerNotification,
	[]ImpactAlternativePlan,
) {
	items := toScoredItems(scored)
	hasCrossRepo := scope.AffectedRepos > 1 || anyReason(items, ImpactReasonCrossRepoContractEdge)
	needsManualReview := aggregate.Confidence < planningLowConfidenceThreshold || hasReasonInSet(items, []ImpactReasonCode{ImpactReasonManualReviewRequired, ImpactReasonPossibleDynamicDispatch}) || len(warnings) > 0
	requiresSplitMigration := hasCrossRepo || aggregate.RiskScore >= planningHighRiskThreshold || scope.DirectCallers >= planningSplitDirectCallerMinimum

	recommendedStrategy := selectImpactStrategy(hasCrossRepo, needsManualReview, requiresSplitMigration, aggregate)
	suggestedBatchCount := computeBatchCount(requiresSplitMigration, hasCrossRepo, needsManualReview, aggregate)
	safeToSinglePR := recommendedStrategy == ImpactStrategySinglePR

	hints := buildAgentHints(hasCrossRepo, needsManualReview, requiresSplitMigration)
	checks := buildRequiredChecks(start, scope, items, aggregate, hasCrossRepo, needsManualReview)
	notifications := buildOwnerNotifications(start, items, hasCrossRepo)
	recommendedOrder := buildRecommendedOrder(recommendedStrategy, checks, notifications, items)
	alternatives := buildAlternatives(recommendedStrategy, aggregate.RiskScore)

	planning := ImpactPlanningSignals{
		RequiresSplitMigration:        requiresSplitMigration,
		RequiresCrossRepoCoordination: hasCrossRepo,
		RecommendedStrategy:           recommendedStrategy,
		SuggestedBatchCount:           suggestedBatchCount,
		SafeToDoInSinglePR:            safeToSinglePR,
		NeedsManualReview:             needsManualReview,
		AgentHints:                    hints,
	}

	return planning, recommendedOrder, checks, notifications, alternatives
}

func toScoredItems(scored []ImpactScoreResult) []ImpactItem {
	items := make([]ImpactItem, 0, len(scored))
	for _, entry := range scored {
		items = append(items, entry.Item)
	}
	return items
}

func selectImpactStrategy(hasCrossRepo, needsManualReview, requiresSplitMigration bool, aggregate ImpactScores) ImpactStrategy {
	if needsManualReview {
		return ImpactStrategyManualFirst
	}
	if hasCrossRepo {
		return ImpactStrategyAdapterBridge
	}
	if aggregate.RiskScore >= planningCriticalRiskThreshold {
		return ImpactStrategyDeprecateRemove
	}
	if requiresSplitMigration {
		return ImpactStrategyAdapterBridge
	}
	return ImpactStrategySinglePR
}

func computeBatchCount(requiresSplitMigration, hasCrossRepo, needsManualReview bool, aggregate ImpactScores) int {
	batches := 1
	if requiresSplitMigration {
		batches++
	}
	if hasCrossRepo {
		batches++
	}
	if aggregate.RiskScore >= planningCriticalRiskThreshold {
		batches++
	}
	if needsManualReview {
		batches++
	}
	if batches > planningMaxBatchCount {
		return planningMaxBatchCount
	}
	return batches
}

func buildAgentHints(hasCrossRepo, needsManualReview, requiresSplitMigration bool) []ImpactAgentHint {
	hints := make([]ImpactAgentHint, 0, 5)
	hints = append(hints, ImpactAgentHintExpandTests)
	if hasCrossRepo {
		hints = append(hints, ImpactAgentHintRunCrossRepoCI, ImpactAgentHintConfirmContractVersion)
	}
	if requiresSplitMigration {
		hints = append(hints, ImpactAgentHintPreferAdapter)
	}
	if needsManualReview {
		hints = append(hints, ImpactAgentHintNotifyOwners)
	}
	return dedupeHints(hints)
}

func dedupeHints(hints []ImpactAgentHint) []ImpactAgentHint {
	seen := map[ImpactAgentHint]struct{}{}
	out := make([]ImpactAgentHint, 0, len(hints))
	for _, hint := range hints {
		if _, ok := seen[hint]; ok {
			continue
		}
		seen[hint] = struct{}{}
		out = append(out, hint)
	}
	return out
}

func buildRequiredChecks(start ImpactSymbolRef, scope ImpactScope, items []ImpactItem, aggregate ImpactScores, hasCrossRepo, needsManualReview bool) []ImpactRequiredCheck {
	checks := []ImpactRequiredCheck{
		{
			ID:       "build-main",
			Type:     ImpactCheckBuild,
			Target:   start.Repo,
			Priority: ImpactPriorityHigh,
			Blocking: true,
			Why:      "Baseline compile validation for impacted repository",
		},
		{
			ID:       "lint-main",
			Type:     ImpactCheckLint,
			Target:   start.Repo,
			Priority: ImpactPriorityMedium,
			Blocking: false,
			Why:      "Static quality checks after API/symbol changes",
		},
	}

	if len(items) > 0 {
		target := fmt.Sprintf("%s:%s", items[0].Target.Repo, items[0].Target.File)
		checks = append(checks, ImpactRequiredCheck{
			ID:       "unit-top-impact",
			Type:     ImpactCheckUnitTest,
			Target:   target,
			Priority: ImpactPriorityHigh,
			Blocking: true,
			Why:      "Top impacted symbol path must be validated",
		})
	}

	if aggregate.RiskScore >= planningHighRiskThreshold || scope.IndirectCallers > 0 {
		checks = append(checks, ImpactRequiredCheck{
			ID:       "integration-impacted-paths",
			Type:     ImpactCheckIntegrationTest,
			Target:   start.Repo,
			Priority: ImpactPriorityHigh,
			Blocking: true,
			Why:      "Indirect caller propagation indicates integration-path risk",
		})
	}

	if hasCrossRepo {
		checks = append(checks, ImpactRequiredCheck{
			ID:       "contract-cross-repo",
			Type:     ImpactCheckContractTest,
			Target:   "workspace",
			Priority: ImpactPriorityHigh,
			Blocking: true,
			Why:      "Cross-repo edges require contract compatibility validation",
		})
	}

	if needsManualReview {
		checks = append(checks, ImpactRequiredCheck{
			ID:       "manual-risk-review",
			Type:     ImpactCheckManualValidation,
			Target:   start.Symbol,
			Priority: ImpactPriorityHigh,
			Blocking: true,
			Why:      "Low-confidence or dynamic paths require explicit reviewer signoff",
		})
	}

	return checks
}

func buildOwnerNotifications(start ImpactSymbolRef, items []ImpactItem, hasCrossRepo bool) []ImpactOwnerNotification {
	notifications := make([]ImpactOwnerNotification, 0)
	seen := map[string]struct{}{}

	for _, item := range items {
		if item.Owner == nil {
			continue
		}
		key := string(item.Owner.Type) + ":" + item.Owner.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		notifications = append(notifications, ImpactOwnerNotification{
			Owner:    *item.Owner,
			Reason:   "Owner mapped from impacted symbol",
			Priority: ImpactPriorityHigh,
		})
	}

	if hasCrossRepo {
		repoIDs := collectAffectedRepoIDs(items)
		for _, repoID := range repoIDs {
			if repoID == "" || repoID == start.Repo {
				continue
			}
			key := string(ImpactOwnerServiceOwner) + ":repo:" + repoID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			notifications = append(notifications, ImpactOwnerNotification{
				Owner: ImpactOwnerRef{
					ID:          "repo:" + repoID,
					Type:        ImpactOwnerServiceOwner,
					DisplayName: "Service owner for " + repoID,
					Confidence:  0.45,
				},
				Reason:   "Cross-repo impact requires coordination",
				Priority: ImpactPriorityHigh,
			})
		}
	}

	sort.Slice(notifications, func(i, j int) bool {
		a, b := notifications[i], notifications[j]
		if a.Priority != b.Priority {
			return string(a.Priority) > string(b.Priority)
		}
		if a.Owner.Type != b.Owner.Type {
			return string(a.Owner.Type) < string(b.Owner.Type)
		}
		return a.Owner.ID < b.Owner.ID
	})
	return notifications
}

func collectAffectedRepoIDs(items []ImpactItem) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0)
	for _, item := range items {
		repoID := item.Target.Repo
		if repoID == "" {
			continue
		}
		if _, ok := seen[repoID]; ok {
			continue
		}
		seen[repoID] = struct{}{}
		ids = append(ids, repoID)
	}
	sort.Strings(ids)
	return ids
}

func buildRecommendedOrder(strategy ImpactStrategy, checks []ImpactRequiredCheck, notifications []ImpactOwnerNotification, items []ImpactItem) []ImpactRecommendedStep {
	steps := make([]ImpactRecommendedStep, 0, 6)
	stepNo := 1

	if len(notifications) > 0 {
		steps = append(steps, ImpactRecommendedStep{
			Step:      stepNo,
			Action:    "Notify impacted owners before implementation",
			Rationale: "Cross-boundary or ownership-sensitive changes should be coordinated early",
			Blocking:  true,
		})
		stepNo++
	}

	if strategy == ImpactStrategyAdapterBridge || strategy == ImpactStrategyDeprecateRemove {
		steps = append(steps, ImpactRecommendedStep{
			Step:      stepNo,
			Action:    "Introduce compatibility adapter/deprecation layer",
			Rationale: "Reduce immediate breakage while downstream callers migrate",
			Blocking:  true,
		})
		stepNo++
	}

	for _, item := range topImpactItems(items, 2) {
		steps = append(steps, ImpactRecommendedStep{
			Step:             stepNo,
			Action:           fmt.Sprintf("Update impacted path %s (%s)", item.Target.Symbol, item.Target.File),
			Rationale:        "Highest-risk impacted symbols should be migrated first",
			RelatedImpactIDs: []string{item.ID},
			Blocking:         false,
		})
		stepNo++
	}

	steps = append(steps, ImpactRecommendedStep{
		Step:      stepNo,
		Action:    fmt.Sprintf("Run %d required checks", len(checks)),
		Rationale: "Validation gates prevent regressions before merge",
		Blocking:  true,
	})

	return steps
}

func topImpactItems(items []ImpactItem, n int) []ImpactItem {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	copyItems := append([]ImpactItem(nil), items...)
	sort.Slice(copyItems, func(i, j int) bool {
		a, b := copyItems[i], copyItems[j]
		if a.RiskScore != b.RiskScore {
			return a.RiskScore > b.RiskScore
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		if a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		return a.ID < b.ID
	})
	if len(copyItems) < n {
		return copyItems
	}
	return copyItems[:n]
}

func buildAlternatives(strategy ImpactStrategy, riskScore int) []ImpactAlternativePlan {
	alternatives := make([]ImpactAlternativePlan, 0, 2)

	if strategy != ImpactStrategyAdapterBridge {
		alternatives = append(alternatives, ImpactAlternativePlan{
			Name:                   "adapter_bridge_then_migrate",
			Description:            "Add a non-breaking compatibility adapter and migrate callers incrementally",
			ExpectedRiskScoreDelta: -20,
			Tradeoffs:              []string{"Extra temporary code", "Longer migration window"},
		})
	}

	if strategy != ImpactStrategyFeatureFlag {
		delta := -10
		if riskScore >= planningCriticalRiskThreshold {
			delta = -18
		}
		alternatives = append(alternatives, ImpactAlternativePlan{
			Name:                   "feature_flag_rollout",
			Description:            "Gate behavior behind a rollout flag and increase exposure gradually",
			ExpectedRiskScoreDelta: delta,
			Tradeoffs:              []string{"Flag lifecycle overhead", "Operational coordination required"},
		})
	}

	return alternatives
}

func hasReasonInSet(items []ImpactItem, reasons []ImpactReasonCode) bool {
	lookup := map[ImpactReasonCode]struct{}{}
	for _, reason := range reasons {
		lookup[reason] = struct{}{}
	}
	for _, item := range items {
		for _, reason := range item.ReasonCodes {
			if _, ok := lookup[reason]; ok {
				return true
			}
		}
	}
	return false
}

func anyReason(items []ImpactItem, target ImpactReasonCode) bool {
	for _, item := range items {
		for _, reason := range item.ReasonCodes {
			if reason == target {
				return true
			}
		}
	}
	return false
}
