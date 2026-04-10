package indexer

// impact_types.go — domain models for counterfactual impact simulation.

// ImpactSchemaVersion is the stable JSON envelope version returned by
// impact simulation in machine-readable mode.
const ImpactSchemaVersion = "impact-sim/v1"

// ImpactChangeKind describes the type of hypothetical change the user intends
// to apply to a symbol.
type ImpactChangeKind string

const (
	ImpactChangeRenameSymbol     ImpactChangeKind = "rename_symbol"
	ImpactChangeAddRequiredParam ImpactChangeKind = "add_required_param"
	ImpactChangeAddOptionalParam ImpactChangeKind = "add_optional_param"
	ImpactChangeRemoveParam      ImpactChangeKind = "remove_param"
	ImpactChangeParamTypeChange  ImpactChangeKind = "param_type_change"
	ImpactChangeReturnTypeChange ImpactChangeKind = "return_type_change"
	ImpactChangeVisibilityChange ImpactChangeKind = "visibility_change"
	ImpactChangeExportChange     ImpactChangeKind = "export_change"
	ImpactChangeBehaviorHint     ImpactChangeKind = "behavior_change_hint"
)

// ImpactRiskTier is a coarse risk classification derived from numeric score.
type ImpactRiskTier string

const (
	ImpactRiskLow      ImpactRiskTier = "low"
	ImpactRiskMedium   ImpactRiskTier = "medium"
	ImpactRiskHigh     ImpactRiskTier = "high"
	ImpactRiskCritical ImpactRiskTier = "critical"
)

// ImpactLikelihood is the qualitative probability of a predicted impact.
type ImpactLikelihood string

const (
	ImpactLikelihoodUnlikely   ImpactLikelihood = "unlikely"
	ImpactLikelihoodPossible   ImpactLikelihood = "possible"
	ImpactLikelihoodLikely     ImpactLikelihood = "likely"
	ImpactLikelihoodVeryLikely ImpactLikelihood = "very_likely"
)

// ImpactBreakType categorizes the likely failure mode for an impacted target.
type ImpactBreakType string

const (
	ImpactBreakCompileError       ImpactBreakType = "compile_error"
	ImpactBreakRuntimeError       ImpactBreakType = "runtime_error"
	ImpactBreakContractBreak      ImpactBreakType = "contract_break"
	ImpactBreakBehaviorRegression ImpactBreakType = "behavioral_regression"
	ImpactBreakTestFailure        ImpactBreakType = "test_failure"
	ImpactBreakUnknown            ImpactBreakType = "unknown"
)

// ImpactReasonCode is a stable taxonomy of explainability signals used by the
// simulator. These constants are intentionally explicit and machine-friendly.
type ImpactReasonCode string

const (
	// ImpactReasonDirectCallerSignatureMismatch indicates a direct caller is
	// likely to fail because the callee signature changed incompatibly.
	ImpactReasonDirectCallerSignatureMismatch ImpactReasonCode = "DIRECT_CALLER_SIGNATURE_MISMATCH"

	// ImpactReasonIndirectCallChainPropagation indicates an impact propagated via
	// transitive call-chain traversal.
	ImpactReasonIndirectCallChainPropagation ImpactReasonCode = "INDIRECT_CALL_CHAIN_PROPAGATION"

	// ImpactReasonReturnTypeIncompatibility indicates return-type changes may
	// break downstream assumptions.
	ImpactReasonReturnTypeIncompatibility ImpactReasonCode = "RETURN_TYPE_INCOMPATIBILITY"

	// ImpactReasonParamTypeIncompatibility indicates parameter type changes may
	// break call sites or coercion expectations.
	ImpactReasonParamTypeIncompatibility ImpactReasonCode = "PARAM_TYPE_INCOMPATIBILITY"

	// ImpactReasonSymbolRenameUnresolvedReference indicates name-based lookups or
	// static references may fail after rename.
	ImpactReasonSymbolRenameUnresolvedReference ImpactReasonCode = "SYMBOL_RENAME_UNRESOLVED_REFERENCE"

	// ImpactReasonVisibilityOrExportBreak indicates changes to export/visibility
	// boundaries may make symbols unreachable.
	ImpactReasonVisibilityOrExportBreak ImpactReasonCode = "VISIBILITY_OR_EXPORT_BREAK"

	// ImpactReasonCrossRepoContractEdge indicates an impact crosses repository
	// boundaries through a declared contract/link.
	ImpactReasonCrossRepoContractEdge ImpactReasonCode = "CROSS_REPO_CONTRACT_EDGE"

	// ImpactReasonEventSchemaMismatchRisk indicates event payload evolution may
	// diverge from consumer expectations.
	ImpactReasonEventSchemaMismatchRisk ImpactReasonCode = "EVENT_SCHEMA_MISMATCH_RISK"

	// ImpactReasonLowTestCoverageZone indicates reduced validation confidence in
	// a part of the graph.
	ImpactReasonLowTestCoverageZone ImpactReasonCode = "LOW_TEST_COVERAGE_ZONE"

	// ImpactReasonHighChurnModule indicates historically volatile code is touched.
	ImpactReasonHighChurnModule ImpactReasonCode = "HIGH_CHURN_MODULE"

	// ImpactReasonHotspotSymbol indicates a highly referenced symbol is impacted.
	ImpactReasonHotspotSymbol ImpactReasonCode = "HOTSPOT_SYMBOL"

	// ImpactReasonNoRuntimeEvidence indicates static-only prediction without
	// runtime corroboration.
	ImpactReasonNoRuntimeEvidence ImpactReasonCode = "NO_RUNTIME_EVIDENCE"

	// ImpactReasonRuntimeCriticalPath indicates runtime-observed critical paths
	// likely intersect the change.
	ImpactReasonRuntimeCriticalPath ImpactReasonCode = "RUNTIME_CRITICAL_PATH"

	// ImpactReasonPossibleDynamicDispatch indicates dynamic dispatch/reflection
	// may hide additional affected targets.
	ImpactReasonPossibleDynamicDispatch ImpactReasonCode = "POSSIBLE_DYNAMIC_DISPATCH"

	// ImpactReasonManualReviewRequired indicates deterministic analysis is
	// insufficient and manual review is advised.
	ImpactReasonManualReviewRequired ImpactReasonCode = "MANUAL_REVIEW_REQUIRED"
)

// ImpactEdgeType identifies what kind of relationship supports one evidence hop.
type ImpactEdgeType string

const (
	ImpactEdgeCall           ImpactEdgeType = "call_edge"
	ImpactEdgeImport         ImpactEdgeType = "import_edge"
	ImpactEdgeContract       ImpactEdgeType = "contract_edge"
	ImpactEdgeWorkspaceLink  ImpactEdgeType = "workspace_link"
	ImpactEdgeOwnership      ImpactEdgeType = "ownership_signal"
	ImpactEdgeTest           ImpactEdgeType = "test_signal"
	ImpactEdgeSymbolRelation ImpactEdgeType = "symbol_relation"
)

// ImpactBoundaryKind describes cross-boundary dependency kinds.
type ImpactBoundaryKind string

const (
	ImpactBoundaryAPIContract    ImpactBoundaryKind = "api_contract"
	ImpactBoundaryEventContract  ImpactBoundaryKind = "event_contract"
	ImpactBoundarySharedType     ImpactBoundaryKind = "shared_type"
	ImpactBoundaryDatabaseBridge ImpactBoundaryKind = "database_contract"
)

// ImpactCheckType describes verification actions emitted by the planner.
type ImpactCheckType string

const (
	ImpactCheckUnitTest         ImpactCheckType = "unit_test"
	ImpactCheckIntegrationTest  ImpactCheckType = "integration_test"
	ImpactCheckContractTest     ImpactCheckType = "contract_test"
	ImpactCheckBuild            ImpactCheckType = "build"
	ImpactCheckLint             ImpactCheckType = "lint"
	ImpactCheckManualValidation ImpactCheckType = "manual_validation"
)

// ImpactPriority is used for check and notification prioritization.
type ImpactPriority string

const (
	ImpactPriorityLow    ImpactPriority = "low"
	ImpactPriorityMedium ImpactPriority = "medium"
	ImpactPriorityHigh   ImpactPriority = "high"
)

// ImpactOwnerType identifies ownership entities for notifications.
type ImpactOwnerType string

const (
	ImpactOwnerTeam         ImpactOwnerType = "team"
	ImpactOwnerUser         ImpactOwnerType = "user"
	ImpactOwnerServiceOwner ImpactOwnerType = "service_owner"
)

// ImpactStrategy is the recommended implementation strategy emitted for agents.
type ImpactStrategy string

const (
	ImpactStrategySinglePR        ImpactStrategy = "single_pr_direct_change"
	ImpactStrategyAdapterBridge   ImpactStrategy = "adapter_bridge_then_migrate"
	ImpactStrategyDeprecateRemove ImpactStrategy = "deprecate_then_remove"
	ImpactStrategyFeatureFlag     ImpactStrategy = "feature_flag_rollout"
	ImpactStrategyManualFirst     ImpactStrategy = "manual_investigation_first"
)

// ImpactAgentHint is a short planner-oriented instruction for LLM agents.
type ImpactAgentHint string

const (
	ImpactAgentHintExpandTests            ImpactAgentHint = "expand_test_plan"
	ImpactAgentHintNotifyOwners           ImpactAgentHint = "notify_owners_before_edit"
	ImpactAgentHintPreferAdapter          ImpactAgentHint = "prefer_non_breaking_adapter"
	ImpactAgentHintRunCrossRepoCI         ImpactAgentHint = "run_cross_repo_ci"
	ImpactAgentHintConfirmContractVersion ImpactAgentHint = "confirm_contract_versioning"
)

// ImpactSimulationResult is the top-level response payload for
// counterfactual impact simulation.
type ImpactSimulationResult struct {
	SchemaVersion      string                    `json:"schema_version"`
	RunID              string                    `json:"run_id"`
	GeneratedAt        string                    `json:"generated_at"`
	Input              ImpactSimulationInput     `json:"input"`
	Scores             ImpactScores              `json:"scores"`
	Scope              ImpactScope               `json:"scope"`
	Summary            ImpactSummary             `json:"summary"`
	Impacts            []ImpactItem              `json:"impacts"`
	HighRiskBoundaries []ImpactBoundary          `json:"high_risk_boundaries"`
	RecommendedOrder   []ImpactRecommendedStep   `json:"recommended_order"`
	RequiredChecks     []ImpactRequiredCheck     `json:"required_checks"`
	NotifyOwners       []ImpactOwnerNotification `json:"notify_owners"`
	Alternatives       []ImpactAlternativePlan   `json:"alternatives"`
	PlanningSignals    ImpactPlanningSignals     `json:"planning_signals"`
	Diagnostics        ImpactDiagnostics         `json:"diagnostics"`
}

// ImpactSimulationInput captures the user-provided simulation query.
type ImpactSimulationInput struct {
	Symbol    string            `json:"symbol"`
	Workspace string            `json:"workspace"`
	Repo      string            `json:"repo,omitempty"`
	Change    ImpactChange      `json:"change"`
	Options   *ImpactOptions    `json:"options,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ImpactChange describes the hypothetical change requested by the user.
type ImpactChange struct {
	Kind           ImpactChangeKind `json:"kind"`
	From           string           `json:"from,omitempty"`
	To             string           `json:"to,omitempty"`
	ParamName      string           `json:"param_name,omitempty"`
	ParamTypeFrom  string           `json:"param_type_from,omitempty"`
	ParamTypeTo    string           `json:"param_type_to,omitempty"`
	ReturnTypeFrom string           `json:"return_type_from,omitempty"`
	ReturnTypeTo   string           `json:"return_type_to,omitempty"`
	Notes          string           `json:"notes,omitempty"`
}

// ImpactOptions tunes simulation breadth and output detail.
type ImpactOptions struct {
	MaxDepth            int  `json:"max_depth,omitempty"`
	IncludeTests        bool `json:"include_tests,omitempty"`
	IncludeCrossRepo    bool `json:"include_cross_repo,omitempty"`
	IncludeAlternatives bool `json:"include_alternatives,omitempty"`
}

// ImpactScores aggregates global risk and confidence values.
type ImpactScores struct {
	RiskScore  int            `json:"risk_score"`
	Confidence float64        `json:"confidence"`
	RiskTier   ImpactRiskTier `json:"risk_tier"`
}

// ImpactScope summarizes how wide the predicted blast radius is.
type ImpactScope struct {
	DirectCallers      int `json:"direct_callers"`
	IndirectCallers    int `json:"indirect_callers"`
	AffectedSymbols    int `json:"affected_symbols"`
	AffectedFiles      int `json:"affected_files"`
	AffectedRepos      int `json:"affected_repos"`
	CrossBoundaryEdges int `json:"cross_boundary_edges"`
}

// ImpactSummary contains concise, human-readable highlights.
type ImpactSummary struct {
	Headline string   `json:"headline"`
	Bullets  []string `json:"bullets"`
}

// ImpactItem is a single ranked prediction entry.
type ImpactItem struct {
	ID               string             `json:"id"`
	Target           ImpactSymbolRef    `json:"target"`
	Distance         int                `json:"distance"`
	RiskScore        int                `json:"risk_score"`
	Confidence       float64            `json:"confidence"`
	RiskTier         ImpactRiskTier     `json:"risk_tier"`
	Likelihood       ImpactLikelihood   `json:"likelihood"`
	BreakTypes       []ImpactBreakType  `json:"break_types"`
	ReasonCodes      []ImpactReasonCode `json:"reason_codes"`
	Evidence         []ImpactEvidence   `json:"evidence"`
	Owner            *ImpactOwnerRef    `json:"owner,omitempty"`
	SuggestedActions []string           `json:"suggested_actions,omitempty"`
}

// ImpactSymbolRef identifies a symbol with repository/file context.
type ImpactSymbolRef struct {
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Repo   string `json:"repo"`
	Line   int    `json:"line,omitempty"`
}

// ImpactEvidence is one relation that supports an impact prediction.
type ImpactEvidence struct {
	Type ImpactEdgeType  `json:"type"`
	From ImpactSymbolRef `json:"from"`
	To   ImpactSymbolRef `json:"to"`
	Meta map[string]any  `json:"meta,omitempty"`
}

// ImpactBoundary describes a high-risk edge that crosses important boundaries.
type ImpactBoundary struct {
	Kind        ImpactBoundaryKind `json:"kind"`
	SourceRepo  string             `json:"source_repo"`
	TargetRepo  string             `json:"target_repo"`
	RiskScore   int                `json:"risk_score"`
	ReasonCodes []ImpactReasonCode `json:"reason_codes"`
}

// ImpactRecommendedStep is one ordered migration/planning action.
type ImpactRecommendedStep struct {
	Step             int      `json:"step"`
	Action           string   `json:"action"`
	Rationale        string   `json:"rationale"`
	RelatedImpactIDs []string `json:"related_impact_ids,omitempty"`
	Blocking         bool     `json:"blocking"`
}

// ImpactRequiredCheck is a concrete verification task to run.
type ImpactRequiredCheck struct {
	ID       string          `json:"id"`
	Type     ImpactCheckType `json:"type"`
	Target   string          `json:"target"`
	Command  string          `json:"command,omitempty"`
	Priority ImpactPriority  `json:"priority"`
	Blocking bool            `json:"blocking"`
	Why      string          `json:"why,omitempty"`
}

// ImpactOwnerRef identifies an owner suggested for coordination.
type ImpactOwnerRef struct {
	ID          string          `json:"id"`
	Type        ImpactOwnerType `json:"type"`
	DisplayName string          `json:"display_name,omitempty"`
	Confidence  float64         `json:"confidence,omitempty"`
}

// ImpactOwnerNotification is a recommended ownership notification.
type ImpactOwnerNotification struct {
	Owner            ImpactOwnerRef `json:"owner"`
	Reason           string         `json:"reason"`
	Priority         ImpactPriority `json:"priority"`
	RelatedImpactIDs []string       `json:"related_impact_ids,omitempty"`
}

// ImpactAlternativePlan is a lower-risk implementation option.
type ImpactAlternativePlan struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	ExpectedRiskScoreDelta int      `json:"expected_risk_score_delta"`
	Tradeoffs              []string `json:"tradeoffs,omitempty"`
}

// ImpactPlanningSignals are compact planner-oriented booleans and strategy hints.
type ImpactPlanningSignals struct {
	RequiresSplitMigration        bool              `json:"requires_split_migration"`
	RequiresCrossRepoCoordination bool              `json:"requires_cross_repo_coordination"`
	RecommendedStrategy           ImpactStrategy    `json:"recommended_strategy"`
	SuggestedBatchCount           int               `json:"suggested_batch_count"`
	SafeToDoInSinglePR            bool              `json:"safe_to_do_in_single_pr"`
	NeedsManualReview             bool              `json:"needs_manual_review"`
	AgentHints                    []ImpactAgentHint `json:"agent_hints,omitempty"`
}

// ImpactDiagnostics captures analysis quality and freshness metadata.
type ImpactDiagnostics struct {
	IndexRefreshed     bool     `json:"index_refreshed"`
	IndexAgeSeconds    int      `json:"index_age_seconds"`
	WorkspaceLinksUsed int      `json:"workspace_links_used"`
	AnalysisDepth      int      `json:"analysis_depth"`
	Warnings           []string `json:"warnings,omitempty"`
}
