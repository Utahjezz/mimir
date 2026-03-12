package indexer

// registry_cache_test.go — tests for the query compilation cache (Task 4).
//
// These white-box tests verify that langEntry and callDefinition hold
// pre-compiled *tree_sitter.Query values after buildLangMap /
// buildCallLangMap, and that the compiled queries are reused correctly
// across multiple symbol-extraction calls on the same MuncherFacade.

import (
	"testing"
)

// --- compile-at-init: langEntry ---

func TestBuildLangMap_CompiledQueryNonNil(t *testing.T) {
	langs := buildLangMap()
	if len(langs) == 0 {
		t.Fatal("buildLangMap returned empty map")
	}
	for ext, entry := range langs {
		if entry.compiledQuery == nil {
			t.Errorf("ext %q: compiledQuery is nil after buildLangMap", ext)
		}
	}
}

// --- compile-at-init: callDefinition ---

func TestBuildCallLangMap_CompiledCallQueryNonNil(t *testing.T) {
	calls := buildCallLangMap()
	if len(calls) == 0 {
		t.Fatal("buildCallLangMap returned empty map")
	}
	for lang, def := range calls {
		if def.compiledCallQuery == nil {
			t.Errorf("lang %q: compiledCallQuery is nil after buildCallLangMap", lang)
		}
		// refQuery may be legitimately empty for some languages, but if
		// the string is non-empty the compiled form must also be non-nil.
		if def.refQuery != "" && def.compiledRefQuery == nil {
			t.Errorf("lang %q: refQuery non-empty but compiledRefQuery is nil", lang)
		}
	}
}

// --- reuse correctness: same muncher, multiple calls ---

func TestCompiledQuery_ReusableAcrossInvocations(t *testing.T) {
	m := NewMuncher()
	code := []byte(jsFixture)

	first, err := m.GetSymbols("file.js", code)
	if err != nil {
		t.Fatalf("first GetSymbols: %v", err)
	}

	for i := 0; i < 5; i++ {
		got, err := m.GetSymbols("file.js", code)
		if err != nil {
			t.Fatalf("GetSymbols invocation %d: %v", i+1, err)
		}
		if len(got) != len(first) {
			t.Errorf("invocation %d: got %d symbols, want %d", i+1, len(got), len(first))
		}
	}
}

func TestCompiledCallQuery_ReusableAcrossInvocations(t *testing.T) {
	goCode := []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello")
	greet("world")
}

func greet(s string) { fmt.Println(s) }
`)

	first, err := ExtractCalls("go", goCode)
	if err != nil {
		t.Fatalf("first ExtractCalls: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected call sites, got none")
	}

	for i := 0; i < 5; i++ {
		got, err := ExtractCalls("go", goCode)
		if err != nil {
			t.Fatalf("ExtractCalls invocation %d: %v", i+1, err)
		}
		if len(got) != len(first) {
			t.Errorf("invocation %d: got %d call sites, want %d", i+1, len(got), len(first))
		}
	}
}
