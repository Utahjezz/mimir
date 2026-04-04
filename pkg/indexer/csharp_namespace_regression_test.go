package indexer

// csharp_namespace_regression_test.go — regression tests proving that
// namespace-qualified searches now work end-to-end (parse → store → search).
//
// Before the fix:
//   - Classes inside namespaces had parent="" (empty)
//   - "Namespace.*" found nothing (no symbol with parent="Namespace")
//   - "*.ClassName" found nothing (wildcard parent requires parent!="", but it was "")
//
// After the fix:
//   - Classes inside namespaces have parent="NamespaceName"
//   - "Namespace.*" returns all classes in that namespace
//   - "*.ClassName" returns the class (parent is non-empty)

import (
	"database/sql"
	"testing"
	"time"
)

// csharpNamespaceFixture simulates a real-world C# project with:
//   - nested namespaces
//   - classes, interfaces, enums at various namespace depths
//   - methods inside classes
const csharpNamespaceFixture = `
namespace Company.GameEngine {
    namespace Cards {
        public class CardBase {
            public string Name { get; set; }
            public virtual void Play() {}
        }

        public class RunnerCard : CardBase {
            public override void Play() {}
            public void Install() {}
        }

        public class CorpCard : CardBase {
            public override void Play() {}
            public void Rez() {}
        }

        public interface IScoreable {
            int Points { get; }
        }

        public enum Faction { Runner, Corp, Neutral }
    }
}
`

// seedCSharpNamespaceDB parses the fixture through the real parser pipeline
// and stores results in a test DB, exactly as `mimir index` would.
func seedCSharpNamespaceDB(t *testing.T) *sql.DB {
	t.Helper()
	m := newTestMuncher()
	symbols, err := m.GetSymbols("cards.cs", []byte(csharpNamespaceFixture))
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}

	db := openTestDB(t, t.TempDir())
	if err := WriteFile(db, "cards.cs", FileEntry{
		Language:  "csharp",
		SHA256:    "test",
		IndexedAt: time.Now().UTC(),
		Symbols:   symbols,
	}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return db
}

// TestCSharpNamespace_Regression_NamespaceWildcard proves that
// "Namespace.*" now finds classes inside that namespace.
//
// BEFORE FIX: 0 results (classes had parent="")
// AFTER FIX:  5 results (CardBase, RunnerCard, CorpCard, IScoreable, Faction)
func TestCSharpNamespace_Regression_NamespaceWildcard(t *testing.T) {
	db := seedCSharpNamespaceDB(t)

	// This is the query that FAILED before the fix.
	// "Company.GameEngine.Cards.*" should find all top-level types in the Cards namespace.
	got, err := SearchSymbols(db, SearchQuery{Name: "Company.GameEngine.Cards.*"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	// Expect: CardBase, RunnerCard, CorpCard, IScoreable, Faction
	if len(got) != 5 {
		t.Errorf("expected 5 results for 'Company.GameEngine.Cards.*', got %d:", len(got))
		for _, r := range got {
			t.Logf("  %s (type=%s, parent=%s)", r.Name, r.Type, r.Parent)
		}
		// Also dump what we actually have in the DB for debugging
		all, _ := SearchSymbols(db, SearchQuery{})
		t.Logf("All symbols in DB:")
		for _, r := range all {
			t.Logf("  %s (type=%s, parent=%q)", r.Name, r.Type, r.Parent)
		}
	}

	// Verify each expected class is present
	found := make(map[string]bool)
	for _, r := range got {
		found[r.Name] = true
	}
	for _, want := range []string{"CardBase", "RunnerCard", "CorpCard", "IScoreable", "Faction"} {
		if !found[want] {
			t.Errorf("expected %q in results, not found", want)
		}
	}
}

// TestCSharpNamespace_Regression_WildcardParent proves that
// "*.ClassName" now finds classes that have a namespace as parent.
//
// BEFORE FIX: 0 results (parent was "" so wildcard parent filter excluded them)
// AFTER FIX:  1 result (RunnerCard with parent="Company.GameEngine.Cards")
func TestCSharpNamespace_Regression_WildcardParent(t *testing.T) {
	db := seedCSharpNamespaceDB(t)

	// "*.RunnerCard" — find RunnerCard in any parent.
	// BEFORE: 0 results (parent was empty, wildcard requires non-empty)
	// AFTER:  1 result  (parent is "Company.GameEngine.Cards")
	got, err := SearchSymbols(db, SearchQuery{Name: "*.RunnerCard"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result for '*.RunnerCard', got %d", len(got))
	}
	if got[0].Parent != "Company.GameEngine.Cards" {
		t.Errorf("RunnerCard parent: got %q, want %q", got[0].Parent, "Company.GameEngine.Cards")
	}
}

// TestCSharpNamespace_Regression_NamespaceType proves that
// "--type namespace" now returns C# namespaces.
//
// BEFORE FIX: 0 results (namespaces were not indexed)
// AFTER FIX:  2 results (Company.GameEngine and Cards)
func TestCSharpNamespace_Regression_NamespaceType(t *testing.T) {
	db := seedCSharpNamespaceDB(t)

	// "--type namespace" should find both namespaces.
	// BEFORE: 0 results
	// AFTER:  2 results (Company.GameEngine, Cards)
	got, err := SearchSymbols(db, SearchQuery{Type: Namespace})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 namespaces, got %d:", len(got))
	}

	names := make(map[string]bool)
	for _, r := range got {
		names[r.Name] = true
		if r.Type != Namespace {
			t.Errorf("expected type namespace, got %s for %s", r.Type, r.Name)
		}
	}
	if !names["Company.GameEngine"] {
		t.Error("expected namespace 'Company.GameEngine'")
	}
	if !names["Cards"] {
		t.Error("expected namespace 'Cards'")
	}
}

// TestCSharpNamespace_Regression_FQNDotNotation proves that
// ParseDotNotation now correctly splits on the last dot for FQN queries.
//
// BEFORE FIX: "A.B.C" → parent="A", name="B.C" (wrong — no match)
// AFTER FIX:  "A.B.C" → parent="A.B", name="C" (correct)
func TestCSharpNamespace_Regression_FQNDotNotation(t *testing.T) {
	db := seedCSharpNamespaceDB(t)

	// "Company.GameEngine.Cards.RunnerCard" — FQN exact match.
	// BEFORE: parent="Company", name="GameEngine.Cards.RunnerCard" → 0 results
	// AFTER:  parent="Company.GameEngine.Cards", name="RunnerCard" → 1 result
	got, err := SearchSymbols(db, SearchQuery{Name: "Company.GameEngine.Cards.RunnerCard"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result for FQN 'Company.GameEngine.Cards.RunnerCard', got %d", len(got))
	}
	if got[0].Name != "RunnerCard" {
		t.Errorf("Name: got %q, want %q", got[0].Name, "RunnerCard")
	}
	if got[0].Parent != "Company.GameEngine.Cards" {
		t.Errorf("Parent: got %q, want %q", got[0].Parent, "Company.GameEngine.Cards")
	}
}

// TestCSharpNamespace_Regression_MethodParentUnchanged proves that
// method-inside-class parent assignment is NOT broken by namespace support.
//
// "CardBase.Play" should still find the Play method with parent=CardBase,
// NOT parent="Company.GameEngine.Cards" (the namespace).
func TestCSharpNamespace_Regression_MethodParentUnchanged(t *testing.T) {
	db := seedCSharpNamespaceDB(t)

	// "CardBase.Play" — method inside class, not namespace.
	got, err := SearchSymbols(db, SearchQuery{Name: "CardBase.Play"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 result for 'CardBase.Play', got %d", len(got))
	}
	if got[0].Parent != "CardBase" {
		t.Errorf("Play parent: got %q, want %q", got[0].Parent, "CardBase")
	}
}
