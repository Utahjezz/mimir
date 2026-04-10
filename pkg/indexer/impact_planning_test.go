package indexer

import "testing"

func TestBuildImpactPlanning_CrossRepoTriggersSplitAndCoordination(t *testing.T) {
	start := ImpactSymbolRef{Repo: "repo-main", File: "svc.go", Symbol: "DoWork"}
	scored := []ImpactScoreResult{
		{
			Item: ImpactItem{
				ID:         "i-1",
				Target:     ImpactSymbolRef{Repo: "repo-main", File: "svc.go", Symbol: "CallerA"},
				RiskScore:  88,
				Confidence: 0.91,
				ReasonCodes: []ImpactReasonCode{
					ImpactReasonDirectCallerSignatureMismatch,
				},
			},
		},
		{
			Item: ImpactItem{
				ID:         "i-2",
				Target:     ImpactSymbolRef{Repo: "repo-remote", File: "handler.go", Symbol: "RemoteConsumer"},
				RiskScore:  82,
				Confidence: 0.85,
				ReasonCodes: []ImpactReasonCode{
					ImpactReasonCrossRepoContractEdge,
				},
			},
		},
	}

	planning, steps, checks, notifications, alternatives := BuildImpactPlanning(
		start,
		ImpactScope{DirectCallers: 3, AffectedRepos: 2, IndirectCallers: 1},
		scored,
		ImpactScores{RiskScore: 86, Confidence: 0.83, RiskTier: ImpactRiskCritical},
		nil,
	)

	if !planning.RequiresSplitMigration {
		t.Fatalf("expected split migration")
	}
	if !planning.RequiresCrossRepoCoordination {
		t.Fatalf("expected cross-repo coordination")
	}
	if planning.RecommendedStrategy != ImpactStrategyAdapterBridge {
		t.Fatalf("expected adapter strategy, got %s", planning.RecommendedStrategy)
	}
	if planning.SuggestedBatchCount < 3 {
		t.Fatalf("expected at least 3 batches, got %d", planning.SuggestedBatchCount)
	}
	if planning.SafeToDoInSinglePR {
		t.Fatalf("expected unsafe single PR for cross-repo high risk")
	}

	if len(steps) == 0 {
		t.Fatalf("expected recommended steps")
	}
	if len(checks) == 0 {
		t.Fatalf("expected required checks")
	}
	if len(notifications) == 0 {
		t.Fatalf("expected owner notifications")
	}
	if len(alternatives) == 0 {
		t.Fatalf("expected alternative plans")
	}
}

func TestBuildImpactPlanning_ManualReviewWhenLowConfidenceOrWarnings(t *testing.T) {
	start := ImpactSymbolRef{Repo: "repo-main", File: "svc.go", Symbol: "DoWork"}
	scored := []ImpactScoreResult{
		{
			Item: ImpactItem{
				ID:         "i-1",
				Target:     ImpactSymbolRef{Repo: "repo-main", File: "svc.go", Symbol: "CallerA"},
				RiskScore:  55,
				Confidence: 0.45,
				ReasonCodes: []ImpactReasonCode{
					ImpactReasonManualReviewRequired,
				},
			},
		},
	}

	planning, _, checks, _, _ := BuildImpactPlanning(
		start,
		ImpactScope{DirectCallers: 1, AffectedRepos: 1},
		scored,
		ImpactScores{RiskScore: 55, Confidence: 0.45, RiskTier: ImpactRiskMedium},
		[]string{"dynamic dispatch edge unresolved"},
	)

	if !planning.NeedsManualReview {
		t.Fatalf("expected manual review true")
	}
	if planning.RecommendedStrategy != ImpactStrategyManualFirst {
		t.Fatalf("expected manual-first strategy, got %s", planning.RecommendedStrategy)
	}
	if !containsAgentHint(planning.AgentHints, ImpactAgentHintNotifyOwners) {
		t.Fatalf("expected notify owners hint when manual review needed")
	}
	if !hasCheckType(checks, ImpactCheckManualValidation) {
		t.Fatalf("expected manual validation check")
	}
}

func TestBuildImpactPlanning_SinglePRLowRisk(t *testing.T) {
	start := ImpactSymbolRef{Repo: "repo-main", File: "svc.go", Symbol: "DoWork"}
	scored := []ImpactScoreResult{
		{
			Item: ImpactItem{
				ID:         "i-1",
				Target:     ImpactSymbolRef{Repo: "repo-main", File: "svc.go", Symbol: "CallerA"},
				RiskScore:  22,
				Confidence: 0.88,
				ReasonCodes: []ImpactReasonCode{
					ImpactReasonIndirectCallChainPropagation,
				},
			},
		},
	}

	planning, _, checks, notifications, _ := BuildImpactPlanning(
		start,
		ImpactScope{DirectCallers: 1, AffectedRepos: 1},
		scored,
		ImpactScores{RiskScore: 24, Confidence: 0.89, RiskTier: ImpactRiskLow},
		nil,
	)

	if planning.RequiresSplitMigration {
		t.Fatalf("did not expect split migration")
	}
	if planning.RequiresCrossRepoCoordination {
		t.Fatalf("did not expect cross-repo coordination")
	}
	if planning.RecommendedStrategy != ImpactStrategySinglePR {
		t.Fatalf("expected single PR strategy, got %s", planning.RecommendedStrategy)
	}
	if !planning.SafeToDoInSinglePR {
		t.Fatalf("expected safe single PR")
	}
	if planning.SuggestedBatchCount != 1 {
		t.Fatalf("expected 1 batch, got %d", planning.SuggestedBatchCount)
	}
	if len(notifications) != 0 {
		t.Fatalf("did not expect owner notifications")
	}
	if len(checks) < 2 {
		t.Fatalf("expected baseline build/lint checks")
	}
}

func containsAgentHint(hints []ImpactAgentHint, target ImpactAgentHint) bool {
	for _, hint := range hints {
		if hint == target {
			return true
		}
	}
	return false
}

func hasCheckType(checks []ImpactRequiredCheck, target ImpactCheckType) bool {
	for _, check := range checks {
		if check.Type == target {
			return true
		}
	}
	return false
}
