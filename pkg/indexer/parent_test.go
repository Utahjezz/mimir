package indexer

// parent_test.go — tests for assignParents (parser post-processing) and
// ParseDotNotation (query dot-split).

import "testing"

// --- assignParents ---

func TestAssignParents_MethodGetsParent(t *testing.T) {
	symbols := []SymbolInfo{
		{Name: "Server", Type: Class, StartLine: 1, EndLine: 20},
		{Name: "serve", Type: Method, StartLine: 5, EndLine: 10},
	}

	got := assignParents(symbols)

	if got[1].Parent != "Server" {
		t.Errorf("serve.Parent: got %q, want %q", got[1].Parent, "Server")
	}
}

func TestAssignParents_TopLevelFunctionNoParent(t *testing.T) {
	symbols := []SymbolInfo{
		{Name: "helper", Type: Function, StartLine: 1, EndLine: 5},
	}

	got := assignParents(symbols)

	if got[0].Parent != "" {
		t.Errorf("helper.Parent: got %q, want empty", got[0].Parent)
	}
}

func TestAssignParents_ClassItselfNoParent(t *testing.T) {
	symbols := []SymbolInfo{
		{Name: "Server", Type: Class, StartLine: 1, EndLine: 20},
	}

	got := assignParents(symbols)

	if got[0].Parent != "" {
		t.Errorf("Server.Parent: got %q, want empty", got[0].Parent)
	}
}

func TestAssignParents_MethodAfterClassEndsNoParent(t *testing.T) {
	symbols := []SymbolInfo{
		{Name: "Server", Type: Class, StartLine: 1, EndLine: 10},
		{Name: "standalone", Type: Function, StartLine: 15, EndLine: 20},
	}

	got := assignParents(symbols)

	if got[1].Parent != "" {
		t.Errorf("standalone.Parent: got %q, want empty", got[1].Parent)
	}
}

func TestAssignParents_MultipleClassesEachOwnMethods(t *testing.T) {
	symbols := []SymbolInfo{
		{Name: "Dog", Type: Class, StartLine: 1, EndLine: 10},
		{Name: "bark", Type: Method, StartLine: 3, EndLine: 6},
		{Name: "Cat", Type: Class, StartLine: 15, EndLine: 25},
		{Name: "meow", Type: Method, StartLine: 17, EndLine: 20},
	}

	got := assignParents(symbols)

	if got[1].Parent != "Dog" {
		t.Errorf("bark.Parent: got %q, want %q", got[1].Parent, "Dog")
	}
	if got[3].Parent != "Cat" {
		t.Errorf("meow.Parent: got %q, want %q", got[3].Parent, "Cat")
	}
}

func TestAssignParents_PropertyGetsParent(t *testing.T) {
	symbols := []SymbolInfo{
		{Name: "User", Type: Class, StartLine: 1, EndLine: 15},
		{Name: "Name", Type: Variable, StartLine: 3, EndLine: 3},
	}

	got := assignParents(symbols)

	if got[1].Parent != "User" {
		t.Errorf("Name.Parent: got %q, want %q", got[1].Parent, "User")
	}
}

// --- ParseDotNotation ---

func TestParseDotNotation_ExactParentAndName(t *testing.T) {
	q := ParseDotNotation(SearchQuery{Name: "Server.serve"})

	if q.Parent != "Server" {
		t.Errorf("Parent: got %q, want %q", q.Parent, "Server")
	}
	if q.Name != "serve" {
		t.Errorf("Name: got %q, want %q", q.Name, "serve")
	}
}

func TestParseDotNotation_WildcardName(t *testing.T) {
	// "Server.*" — all members of Server
	q := ParseDotNotation(SearchQuery{Name: "Server.*"})

	if q.Parent != "Server" {
		t.Errorf("Parent: got %q, want %q", q.Parent, "Server")
	}
	if q.Name != "" {
		t.Errorf("Name: got %q, want empty", q.Name)
	}
}

func TestParseDotNotation_WildcardParent(t *testing.T) {
	// "*.serve" — method named serve on any class
	q := ParseDotNotation(SearchQuery{Name: "*.serve"})

	if q.Parent != "*" {
		t.Errorf("Parent: got %q, want %q", q.Parent, "*")
	}
	if q.Name != "serve" {
		t.Errorf("Name: got %q, want %q", q.Name, "serve")
	}
}

func TestParseDotNotation_LikeWithParent(t *testing.T) {
	// "Server.se" via NameLike — parent exact, name prefix
	q := ParseDotNotation(SearchQuery{NameLike: "Server.se"})

	if q.Parent != "Server" {
		t.Errorf("Parent: got %q, want %q", q.Parent, "Server")
	}
	if q.NameLike != "se" {
		t.Errorf("NameLike: got %q, want %q", q.NameLike, "se")
	}
}

func TestParseDotNotation_NoDot_Unchanged(t *testing.T) {
	q := ParseDotNotation(SearchQuery{Name: "serve"})

	if q.Parent != "" {
		t.Errorf("Parent: got %q, want empty", q.Parent)
	}
	if q.Name != "serve" {
		t.Errorf("Name: got %q, want %q", q.Name, "serve")
	}
}

// --- end-to-end parent assignment via GetSymbols ---

func TestGetSymbols_JS_MethodGetsParent(t *testing.T) {
	m := newTestMuncher()
	src := []byte(`
class Animal {
  constructor(name) { this.name = name; }
  speak() { return this.name; }
}
`)
	symbols, err := m.GetSymbols("animal.js", src)
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}

	byName := byNameMap(symbols)

	speak, ok := byName["speak"]
	if !ok {
		t.Fatal("expected symbol 'speak'")
	}
	if speak.Parent != "Animal" {
		t.Errorf("speak.Parent: got %q, want %q", speak.Parent, "Animal")
	}

	// Top-level class itself has no parent.
	animal, ok := byName["Animal"]
	if !ok {
		t.Fatal("expected symbol 'Animal'")
	}
	if animal.Parent != "" {
		t.Errorf("Animal.Parent: got %q, want empty", animal.Parent)
	}
}

func TestGetSymbols_TS_MethodGetsParent(t *testing.T) {
	m := newTestMuncher()
	src := []byte(`
class Server {
  serve(req: Request): Response { return new Response(); }
  stop(): void {}
}
function standalone(): void {}
`)
	symbols, err := m.GetSymbols("server.ts", src)
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}

	byName := byNameMap(symbols)

	for _, methodName := range []string{"serve", "stop"} {
		s, ok := byName[methodName]
		if !ok {
			t.Fatalf("expected symbol %q", methodName)
		}
		if s.Parent != "Server" {
			t.Errorf("%s.Parent: got %q, want %q", methodName, s.Parent, "Server")
		}
	}

	// Top-level function has no parent.
	standalone := byName["standalone"]
	if standalone.Parent != "" {
		t.Errorf("standalone.Parent: got %q, want empty", standalone.Parent)
	}
}

func TestGetSymbols_Python_MethodGetsParent(t *testing.T) {
	m := newTestMuncher()
	src := []byte(`
class Dog:
    def bark(self):
        return "woof"

def helper():
    pass
`)
	symbols, err := m.GetSymbols("dog.py", src)
	if err != nil {
		t.Fatalf("GetSymbols: %v", err)
	}

	byName := byNameMap(symbols)

	bark, ok := byName["bark"]
	if !ok {
		t.Fatal("expected symbol 'bark'")
	}
	if bark.Parent != "Dog" {
		t.Errorf("bark.Parent: got %q, want %q", bark.Parent, "Dog")
	}

	helper, ok := byName["helper"]
	if !ok {
		t.Fatal("expected symbol 'helper'")
	}
	if helper.Parent != "" {
		t.Errorf("helper.Parent: got %q, want empty", helper.Parent)
	}
}
