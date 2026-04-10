package indexer

import "testing"

func TestScoreImpactItems_AssignsScoresAndTiers(t *testing.T) {
	items := []ImpactItem{
		{
			ID:         "a",
			Target:     ImpactSymbolRef{Repo: "repoA", File: "a.go", Symbol: "alpha"},
			Distance:   1,
			BreakTypes: []ImpactBreakType{ImpactBreakCompileError},
			ReasonCodes: []ImpactReasonCode{
				ImpactReasonDirectCallerSignatureMismatch,
			},
		},
		{
			ID:         "b",
			Target:     ImpactSymbolRef{Repo: "repoA", File: "b.go", Symbol: "beta"},
			Distance:   3,
			BreakTypes: []ImpactBreakType{ImpactBreakBehaviorRegression},
			ReasonCodes: []ImpactReasonCode{
				ImpactReasonIndirectCallChainPropagation,
			},
		},
	}

	results, aggregate := ScoreImpactItems(items, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 scored items, got %d", len(results))
	}

	if results[0].Item.Target.Symbol != "alpha" {
		t.Fatalf("expected alpha to rank first, got %s", results[0].Item.Target.Symbol)
	}

	for _, result := range results {
		if result.Item.RiskScore < 0 || result.Item.RiskScore > 100 {
			t.Fatalf("risk score out of range: %d", result.Item.RiskScore)
		}
		if result.Item.Confidence < 0 || result.Item.Confidence > 1 {
			t.Fatalf("confidence out of range: %.3f", result.Item.Confidence)
		}
		if result.Item.RiskTier == "" {
			t.Fatalf("risk tier not set")
		}
	}

	if aggregate.RiskScore < 0 || aggregate.RiskScore > 100 {
		t.Fatalf("aggregate risk score out of range: %d", aggregate.RiskScore)
	}
	if aggregate.Confidence < 0 || aggregate.Confidence > 1 {
		t.Fatalf("aggregate confidence out of range: %.3f", aggregate.Confidence)
	}
}

func TestScoreImpactItems_CrossRepoGetsBonus(t *testing.T) {
	base := ImpactItem{
		ID:         "base",
		Target:     ImpactSymbolRef{Repo: "repoA", File: "a.go", Symbol: "svc"},
		Distance:   1,
		BreakTypes: []ImpactBreakType{ImpactBreakContractBreak},
		ReasonCodes: []ImpactReasonCode{
			ImpactReasonCrossRepoContractEdge,
		},
	}

	withoutCross := base
	withoutCross.ReasonCodes = []ImpactReasonCode{ImpactReasonIndirectCallChainPropagation}

	withResults, _ := ScoreImpactItems([]ImpactItem{base}, nil)
	withoutResults, _ := ScoreImpactItems([]ImpactItem{withoutCross}, nil)

	if len(withResults) != 1 || len(withoutResults) != 1 {
		t.Fatalf("expected single result for each scoring run")
	}
	if withResults[0].Item.RiskScore <= withoutResults[0].Item.RiskScore {
		t.Fatalf("expected cross-repo case to score higher: with=%d without=%d", withResults[0].Item.RiskScore, withoutResults[0].Item.RiskScore)
	}
}

func TestScoreImpactItems_DeterministicOrdering(t *testing.T) {
	items := []ImpactItem{
		{
			ID:          "z",
			Target:      ImpactSymbolRef{Repo: "repoB", File: "b.go", Symbol: "zeta", Line: 20},
			Distance:    2,
			BreakTypes:  []ImpactBreakType{ImpactBreakBehaviorRegression},
			ReasonCodes: []ImpactReasonCode{ImpactReasonIndirectCallChainPropagation},
		},
		{
			ID:          "a",
			Target:      ImpactSymbolRef{Repo: "repoA", File: "a.go", Symbol: "alpha", Line: 10},
			Distance:    2,
			BreakTypes:  []ImpactBreakType{ImpactBreakBehaviorRegression},
			ReasonCodes: []ImpactReasonCode{ImpactReasonIndirectCallChainPropagation},
		},
	}

	first, _ := ScoreImpactItems(items, nil)
	second, _ := ScoreImpactItems(items, nil)

	if len(first) != len(second) {
		t.Fatalf("scoring result size mismatch: %d vs %d", len(first), len(second))
	}

	for i := range first {
		if first[i].Item.Target.Repo != second[i].Item.Target.Repo ||
			first[i].Item.Target.File != second[i].Item.Target.File ||
			first[i].Item.Target.Symbol != second[i].Item.Target.Symbol ||
			first[i].Item.RiskScore != second[i].Item.RiskScore ||
			first[i].Item.Confidence != second[i].Item.Confidence {
			t.Fatalf("non-deterministic score ordering at index %d", i)
		}
	}
}

func TestScoreToRiskTierThresholds(t *testing.T) {
	tests := []struct {
		score int
		want  ImpactRiskTier
	}{
		{0, ImpactRiskLow},
		{34, ImpactRiskLow},
		{35, ImpactRiskMedium},
		{64, ImpactRiskMedium},
		{65, ImpactRiskHigh},
		{84, ImpactRiskHigh},
		{85, ImpactRiskCritical},
		{100, ImpactRiskCritical},
	}

	for _, tt := range tests {
		got := scoreToRiskTier(tt.score)
		if got != tt.want {
			t.Fatalf("scoreToRiskTier(%d) = %s, want %s", tt.score, got, tt.want)
		}
	}
}
