package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Utahjezz/mimir/pkg/indexer"
	"github.com/spf13/cobra"
)

func TestWriteImpactResultJSON_UsesStableEnvelopeAndArrays(t *testing.T) {
	cmd := &cobra.Command{}
	out := new(strings.Builder)
	cmd.SetOut(out)

	result := indexer.ImpactSimulationResult{
		SchemaVersion: indexer.ImpactSchemaVersion,
		RunID:         "impact-test",
		GeneratedAt:   "2026-01-01T00:00:00Z",
		Input: indexer.ImpactSimulationInput{
			Symbol:    "runImpact",
			Workspace: "default",
			Repo:      ".",
			Change: indexer.ImpactChange{
				Kind: indexer.ImpactChangeRenameSymbol,
				To:   "runImpactV2",
			},
		},
		Scores: indexer.ImpactScores{RiskScore: 42, Confidence: 0.77, RiskTier: indexer.ImpactRiskMedium},
		Scope:  indexer.ImpactScope{AffectedSymbols: 1, AffectedFiles: 1, AffectedRepos: 1},
		Summary: indexer.ImpactSummary{
			Headline: "Impact simulation for runImpact",
		},
		PlanningSignals: indexer.ImpactPlanningSignals{
			RecommendedStrategy: indexer.ImpactStrategySinglePR,
		},
		Diagnostics: indexer.ImpactDiagnostics{},
	}

	if err := writeImpactResultJSON(cmd, result); err != nil {
		t.Fatalf("writeImpactResultJSON: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out.String()), &parsed); err != nil {
		t.Fatalf("json unmarshal: %v\nraw=%s", err, out.String())
	}

	if got := parsed["schema_version"]; got != indexer.ImpactSchemaVersion {
		t.Fatalf("schema_version = %v, want %s", got, indexer.ImpactSchemaVersion)
	}

	for _, key := range []string{"impacts", "high_risk_boundaries", "recommended_order", "required_checks", "notify_owners", "alternatives"} {
		v, ok := parsed[key]
		if !ok {
			t.Fatalf("missing key %q", key)
		}
		if _, isArray := v.([]any); !isArray {
			t.Fatalf("expected %q to be JSON array, got %T", key, v)
		}
	}
}

func TestWriteImpactResultHuman_IncludesKeySections(t *testing.T) {
	cmd := &cobra.Command{}
	out := new(strings.Builder)
	cmd.SetOut(out)

	result := indexer.ImpactSimulationResult{
		SchemaVersion: indexer.ImpactSchemaVersion,
		Summary: indexer.ImpactSummary{
			Headline: "Impact simulation for runImpact",
			Bullets:  []string{"Risk score 72 (high), confidence 0.81"},
		},
		Scores: indexer.ImpactScores{RiskScore: 72, Confidence: 0.81, RiskTier: indexer.ImpactRiskHigh},
		Scope:  indexer.ImpactScope{DirectCallers: 2, IndirectCallers: 1, AffectedSymbols: 3, AffectedFiles: 2, AffectedRepos: 1},
		Impacts: []indexer.ImpactItem{
			{
				ID:         "i1",
				Target:     indexer.ImpactSymbolRef{Symbol: "service", File: "svc.go", Repo: "repo-main", Line: 11},
				RiskScore:  80,
				RiskTier:   indexer.ImpactRiskHigh,
				Confidence: 0.9,
				Distance:   1,
				ReasonCodes: []indexer.ImpactReasonCode{
					indexer.ImpactReasonDirectCallerSignatureMismatch,
				},
			},
		},
		RequiredChecks:   []indexer.ImpactRequiredCheck{{ID: "build-main", Type: indexer.ImpactCheckBuild, Priority: indexer.ImpactPriorityHigh, Blocking: true, Target: "repo-main"}},
		RecommendedOrder: []indexer.ImpactRecommendedStep{{Step: 1, Action: "Notify owners"}},
		NotifyOwners: []indexer.ImpactOwnerNotification{{
			Owner:  indexer.ImpactOwnerRef{ID: "repo:repo-main", Type: indexer.ImpactOwnerServiceOwner},
			Reason: "Cross-repo impact requires coordination",
		}},
		Diagnostics: indexer.ImpactDiagnostics{Warnings: []string{"sample warning"}},
	}

	if err := writeImpactResultHuman(cmd, result); err != nil {
		t.Fatalf("writeImpactResultHuman: %v", err)
	}

	text := out.String()
	for _, snippet := range []string{
		"Impact simulation for runImpact",
		"Risk:",
		"Top impacts:",
		"Required checks:",
		"Recommended order:",
		"Notify owners:",
		"Warnings:",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected output to contain %q\n---\n%s", snippet, text)
		}
	}
}

func TestBuildImpactSummary_IncludesCrossBoundaryBullet(t *testing.T) {
	summary := buildImpactSummary(
		indexer.ImpactSymbolRef{Symbol: "runImpact"},
		indexer.ImpactScores{RiskScore: 90, Confidence: 0.93, RiskTier: indexer.ImpactRiskCritical},
		indexer.ImpactScope{AffectedSymbols: 5, AffectedFiles: 3, AffectedRepos: 2, CrossBoundaryEdges: 2},
	)

	if summary.Headline != "Impact simulation for runImpact" {
		t.Fatalf("unexpected headline: %s", summary.Headline)
	}
	foundCross := false
	for _, bullet := range summary.Bullets {
		if strings.Contains(bullet, "Cross-boundary edges detected") {
			foundCross = true
			break
		}
	}
	if !foundCross {
		t.Fatalf("expected cross-boundary bullet in summary")
	}
}

func TestRunImpactSimulate_JSONFixtureScenarios(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	root := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	code := `package fixture

func runImpact() {}

func service() {
	runImpact()
}

func controller() {
	service()
}
`
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(code), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := indexer.OpenIndex(root)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	if _, err := indexer.Run(root, db); err != nil {
		t.Fatalf("run index: %v", err)
	}
	_ = db.Close()

	oldSymbol, oldChange, oldMaxDepth := impactSimulateSymbol, impactSimulateChangeRaw, impactSimulateMaxDepth
	oldCrossRepo, oldJSON, oldNoRefresh := impactSimulateCrossRepo, impactSimulateJSON, impactSimulateNoRefresh
	oldWorkspace := impactSimulateWorkspace
	t.Cleanup(func() {
		impactSimulateSymbol = oldSymbol
		impactSimulateChangeRaw = oldChange
		impactSimulateMaxDepth = oldMaxDepth
		impactSimulateCrossRepo = oldCrossRepo
		impactSimulateJSON = oldJSON
		impactSimulateNoRefresh = oldNoRefresh
		impactSimulateWorkspace = oldWorkspace
	})

	scenarios := []struct {
		name   string
		change string
	}{
		{name: "local-only rename", change: "rename_symbol:to=runImpactV2"},
		{name: "low-confidence chain via behavior hint", change: "behavior_change_hint:notes=potential semantic change"},
		{name: "signature shift", change: "add_required_param:param_name=currency:param_type_to=string"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Arrange
			impactSimulateSymbol = "runImpact"
			impactSimulateChangeRaw = scenario.change
			impactSimulateMaxDepth = 4
			impactSimulateCrossRepo = false
			impactSimulateJSON = true
			impactSimulateNoRefresh = false
			impactSimulateWorkspace = ""

			cmd := &cobra.Command{}
			stdout := new(bytes.Buffer)
			cmd.SetOut(stdout)

			// Act
			err := runImpactSimulate(cmd, []string{root})
			if err != nil {
				t.Fatalf("runImpactSimulate: %v", err)
			}

			// Assert
			var payload map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal output: %v\nraw=%s", err, stdout.String())
			}

			if payload["schema_version"] != indexer.ImpactSchemaVersion {
				t.Fatalf("schema_version=%v want %s", payload["schema_version"], indexer.ImpactSchemaVersion)
			}

			for _, key := range []string{"scores", "scope", "impacts", "required_checks", "recommended_order", "planning_signals", "diagnostics"} {
				if _, ok := payload[key]; !ok {
					t.Fatalf("missing key %q", key)
				}
			}
		})
	}
}

func TestImpactHelpTextMentionsSimulationOutputs(t *testing.T) {
	if !strings.Contains(impactSimulateCmd.Long, "impact-sim/v1") {
		t.Fatalf("expected help text to mention impact-sim/v1 contract")
	}
	if !strings.Contains(impactSimulateCmd.Long, "Text mode emits a concise human-oriented summary") {
		t.Fatalf("expected help text to mention human-oriented summary mode")
	}
}
