package indexer

import "sort"

// impact_scoring.go — explainable risk scoring for propagated impact entries.

// ImpactScoringConfig controls thresholds and weighted contributors.
type ImpactScoringConfig struct {
	BaseByReason           map[ImpactReasonCode]int `json:"base_by_reason,omitempty"`
	BreakTypeWeight        map[ImpactBreakType]int  `json:"break_type_weight,omitempty"`
	DistancePenaltyStep    int                      `json:"distance_penalty_step,omitempty"`
	DistancePenaltyCap     int                      `json:"distance_penalty_cap,omitempty"`
	CrossRepoBonus         int                      `json:"cross_repo_bonus,omitempty"`
	MinConfidence          float64                  `json:"min_confidence,omitempty"`
	MaxConfidence          float64                  `json:"max_confidence,omitempty"`
	ConfidenceDistanceStep float64                  `json:"confidence_distance_step,omitempty"`
	ScoreBoostScale        float64                  `json:"score_boost_scale,omitempty"`
}

// ImpactScoringExplanation captures the score build-up for an impact item.
type ImpactScoringExplanation struct {
	BaseByReasons      map[ImpactReasonCode]int `json:"base_by_reasons"`
	BreakTypeWeightSum int                      `json:"break_type_weight_sum"`
	DistancePenalty    int                      `json:"distance_penalty"`
	CrossRepoBonus     int                      `json:"cross_repo_bonus"`
	RawScore           int                      `json:"raw_score"`
	FinalScore         int                      `json:"final_score"`
	Confidence         float64                  `json:"confidence"`
	RiskTier           ImpactRiskTier           `json:"risk_tier"`
}

// ImpactScoreResult is the scored outcome for a single impact item.
type ImpactScoreResult struct {
	Item        ImpactItem               `json:"item"`
	Explanation ImpactScoringExplanation `json:"explanation"`
}

// ScoreImpactItems assigns per-item risk score, confidence, and risk tier.
// It returns deterministic, score-desc sorted results plus an aggregate score block.
func ScoreImpactItems(items []ImpactItem, cfg *ImpactScoringConfig) ([]ImpactScoreResult, ImpactScores) {
	config := defaultImpactScoringConfig()
	if cfg != nil {
		config = mergeImpactScoringConfig(config, *cfg)
	}

	results := make([]ImpactScoreResult, 0, len(items))
	for _, item := range items {
		scored, explanation := scoreSingleImpact(item, config)
		results = append(results, ImpactScoreResult{Item: scored, Explanation: explanation})
	}

	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Item.RiskScore != b.Item.RiskScore {
			return a.Item.RiskScore > b.Item.RiskScore
		}
		if a.Item.Confidence != b.Item.Confidence {
			return a.Item.Confidence > b.Item.Confidence
		}
		if a.Item.Distance != b.Item.Distance {
			return a.Item.Distance < b.Item.Distance
		}
		if a.Item.Target.Repo != b.Item.Target.Repo {
			return a.Item.Target.Repo < b.Item.Target.Repo
		}
		if a.Item.Target.File != b.Item.Target.File {
			return a.Item.Target.File < b.Item.Target.File
		}
		if a.Item.Target.Symbol != b.Item.Target.Symbol {
			return a.Item.Target.Symbol < b.Item.Target.Symbol
		}
		return a.Item.Target.Line < b.Item.Target.Line
	})

	agg := aggregateImpactScores(results)
	return results, agg
}

func scoreSingleImpact(item ImpactItem, cfg ImpactScoringConfig) (ImpactItem, ImpactScoringExplanation) {
	scored := item

	baseByReasons := make(map[ImpactReasonCode]int)
	baseSum := 0
	for _, reason := range scored.ReasonCodes {
		weight := cfg.BaseByReason[reason]
		if weight <= 0 {
			continue
		}
		baseByReasons[reason] = weight
		baseSum += weight
	}

	breakSum := 0
	for _, bt := range scored.BreakTypes {
		weight := cfg.BreakTypeWeight[bt]
		if weight <= 0 {
			continue
		}
		breakSum += weight
	}

	distancePenalty := distancePenalty(scored.Distance, cfg)
	crossRepoBonus := crossRepoBonus(scored)
	raw := baseSum + breakSum + crossRepoBonus - distancePenalty
	final := clampInt(raw, 0, 100)
	conf := confidenceForImpact(scored, final, cfg)
	tier := scoreToRiskTier(final)
	likely := scoreToLikelihood(final)

	scored.RiskScore = final
	scored.Confidence = conf
	scored.RiskTier = tier
	scored.Likelihood = likely

	explanation := ImpactScoringExplanation{
		BaseByReasons:      baseByReasons,
		BreakTypeWeightSum: breakSum,
		DistancePenalty:    distancePenalty,
		CrossRepoBonus:     crossRepoBonus,
		RawScore:           raw,
		FinalScore:         final,
		Confidence:         conf,
		RiskTier:           tier,
	}

	return scored, explanation
}

func aggregateImpactScores(results []ImpactScoreResult) ImpactScores {
	if len(results) == 0 {
		return ImpactScores{RiskScore: 0, Confidence: 0, RiskTier: ImpactRiskLow}
	}

	top := results[0].Item.RiskScore
	if top <= 0 {
		top = 0
	}
	if top > 100 {
		top = 100
	}

	weightedRisk := 0.0
	weightSum := 0.0
	confidenceSum := 0.0
	for _, result := range results {
		weight := 1.0
		if result.Item.Distance > 1 {
			weight = 1.0 / float64(result.Item.Distance)
		}
		weightedRisk += float64(result.Item.RiskScore) * weight
		weightSum += weight
		confidenceSum += result.Item.Confidence
	}

	avgRisk := 0
	if weightSum > 0 {
		avgRisk = clampInt(int(weightedRisk/weightSum+0.5), 0, 100)
	}
	agg := clampInt((top*60+avgRisk*40)/100, 0, 100)
	avgConfidence := confidenceSum / float64(len(results))

	return ImpactScores{
		RiskScore:  agg,
		Confidence: clampFloat(avgConfidence, 0, 1),
		RiskTier:   scoreToRiskTier(agg),
	}
}

func defaultImpactScoringConfig() ImpactScoringConfig {
	return ImpactScoringConfig{
		BaseByReason: map[ImpactReasonCode]int{
			ImpactReasonDirectCallerSignatureMismatch:   45,
			ImpactReasonCrossRepoContractEdge:           35,
			ImpactReasonReturnTypeIncompatibility:       30,
			ImpactReasonParamTypeIncompatibility:        28,
			ImpactReasonSymbolRenameUnresolvedReference: 26,
			ImpactReasonVisibilityOrExportBreak:         24,
			ImpactReasonEventSchemaMismatchRisk:         22,
			ImpactReasonIndirectCallChainPropagation:    18,
			ImpactReasonHighChurnModule:                 10,
			ImpactReasonHotspotSymbol:                   10,
			ImpactReasonLowTestCoverageZone:             8,
			ImpactReasonNoRuntimeEvidence:               -8,
			ImpactReasonRuntimeCriticalPath:             12,
			ImpactReasonPossibleDynamicDispatch:         8,
			ImpactReasonManualReviewRequired:            6,
		},
		BreakTypeWeight: map[ImpactBreakType]int{
			ImpactBreakCompileError:       22,
			ImpactBreakContractBreak:      20,
			ImpactBreakRuntimeError:       18,
			ImpactBreakBehaviorRegression: 14,
			ImpactBreakTestFailure:        8,
			ImpactBreakUnknown:            4,
		},
		DistancePenaltyStep:    4,
		DistancePenaltyCap:     24,
		CrossRepoBonus:         12,
		MinConfidence:          0.35,
		MaxConfidence:          0.98,
		ConfidenceDistanceStep: 0.08,
		ScoreBoostScale:        0.003,
	}
}

func mergeImpactScoringConfig(base, override ImpactScoringConfig) ImpactScoringConfig {
	merged := base

	if len(override.BaseByReason) > 0 {
		merged.BaseByReason = copyReasonWeights(base.BaseByReason)
		for reason, weight := range override.BaseByReason {
			merged.BaseByReason[reason] = weight
		}
	}

	if len(override.BreakTypeWeight) > 0 {
		merged.BreakTypeWeight = copyBreakWeights(base.BreakTypeWeight)
		for breakType, weight := range override.BreakTypeWeight {
			merged.BreakTypeWeight[breakType] = weight
		}
	}

	if override.DistancePenaltyStep != 0 {
		merged.DistancePenaltyStep = override.DistancePenaltyStep
	}
	if override.DistancePenaltyCap != 0 {
		merged.DistancePenaltyCap = override.DistancePenaltyCap
	}
	if override.CrossRepoBonus != 0 {
		merged.CrossRepoBonus = override.CrossRepoBonus
	}
	if override.MinConfidence != 0 {
		merged.MinConfidence = override.MinConfidence
	}
	if override.MaxConfidence != 0 {
		merged.MaxConfidence = override.MaxConfidence
	}
	if override.ConfidenceDistanceStep != 0 {
		merged.ConfidenceDistanceStep = override.ConfidenceDistanceStep
	}
	if override.ScoreBoostScale != 0 {
		merged.ScoreBoostScale = override.ScoreBoostScale
	}

	return merged
}

func copyReasonWeights(src map[ImpactReasonCode]int) map[ImpactReasonCode]int {
	dst := make(map[ImpactReasonCode]int, len(src))
	for reason, weight := range src {
		dst[reason] = weight
	}
	return dst
}

func copyBreakWeights(src map[ImpactBreakType]int) map[ImpactBreakType]int {
	dst := make(map[ImpactBreakType]int, len(src))
	for breakType, weight := range src {
		dst[breakType] = weight
	}
	return dst
}

func distancePenalty(distance int, cfg ImpactScoringConfig) int {
	if distance <= 1 {
		return 0
	}
	penalty := (distance - 1) * cfg.DistancePenaltyStep
	if penalty < 0 {
		penalty = 0
	}
	if cfg.DistancePenaltyCap > 0 && penalty > cfg.DistancePenaltyCap {
		return cfg.DistancePenaltyCap
	}
	return penalty
}

func crossRepoBonus(item ImpactItem) int {
	for _, reason := range item.ReasonCodes {
		if reason == ImpactReasonCrossRepoContractEdge {
			return 12
		}
	}
	return 0
}

func confidenceForImpact(item ImpactItem, score int, cfg ImpactScoringConfig) float64 {
	confidence := cfg.MaxConfidence
	if item.Distance > 1 {
		confidence -= float64(item.Distance-1) * cfg.ConfidenceDistanceStep
	}
	confidence += float64(score) * cfg.ScoreBoostScale
	if containsReason(item.ReasonCodes, ImpactReasonNoRuntimeEvidence) {
		confidence -= 0.1
	}
	if containsReason(item.ReasonCodes, ImpactReasonManualReviewRequired) {
		confidence -= 0.05
	}
	return clampFloat(confidence, cfg.MinConfidence, cfg.MaxConfidence)
}

func containsReason(reasons []ImpactReasonCode, target ImpactReasonCode) bool {
	for _, reason := range reasons {
		if reason == target {
			return true
		}
	}
	return false
}

func scoreToRiskTier(score int) ImpactRiskTier {
	switch {
	case score >= 85:
		return ImpactRiskCritical
	case score >= 65:
		return ImpactRiskHigh
	case score >= 35:
		return ImpactRiskMedium
	default:
		return ImpactRiskLow
	}
}

func scoreToLikelihood(score int) ImpactLikelihood {
	switch {
	case score >= 85:
		return ImpactLikelihoodVeryLikely
	case score >= 60:
		return ImpactLikelihoodLikely
	case score >= 30:
		return ImpactLikelihoodPossible
	default:
		return ImpactLikelihoodUnlikely
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
