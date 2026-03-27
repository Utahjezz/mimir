# C# Namespace Indexing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Index C# namespace declarations as symbols and use them as parent containers so that `--name "Namespace.*"` queries find classes within namespaces.

**Architecture:** Extend existing `assignParents` stack mechanism to treat namespaces as parent containers with FQN concatenation for nested namespaces. Change `ParseDotNotation` to split on last dot for correct FQN queries.

**Tech Stack:** Go, tree-sitter (C# grammar), SQLite

**Spec:** `docs/specs/2026-03-27-csharp-namespace-indexing-design.md`

---

## Chunk 1: C# Query Patterns + Parser Changes

### Task 1: Add namespace query patterns to C# grammar

**Files:**
- Modify: `pkg/indexer/languages/csharp/queries.go:41-65`
- Test: `pkg/indexer/csharp_test.go`

- [ ] **Step 1: Write failing test for simple namespace extraction**

Add to `pkg/indexer/csharp_test.go`:

```go
func TestGetSymbols_CSharp_Namespace_Simple(t *testing.T) {
	const fixture = `
namespace DataAccess {
    public class UserRepository {
        public void Save() {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byName := byNameMap(symbols)
	s, ok := byName["DataAccess"]
	if !ok {
		t.Fatal(`symbol "DataAccess" not found`)
	}
	if s.Type != Namespace {
		t.Errorf(`"DataAccess": got type %q, want %q`, s.Type, Namespace)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestGetSymbols_CSharp_Namespace_Simple -v`
Expected: FAIL — `symbol "DataAccess" not found`

- [ ] **Step 3: Add namespace patterns to C# queries**

In `pkg/indexer/languages/csharp/queries.go`, append to the `Queries` constant (before the closing backtick):

```go
(namespace_declaration
  name: (qualified_name) @name) @namespace

(namespace_declaration
  name: (identifier) @name) @namespace

(file_scoped_namespace_declaration
  name: (qualified_name) @name) @namespace

(file_scoped_namespace_declaration
  name: (identifier) @name) @namespace
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestGetSymbols_CSharp_Namespace_Simple -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/indexer/languages/csharp/queries.go pkg/indexer/csharp_test.go
git commit -m "feat(csharp): add tree-sitter query patterns for namespace declarations"
```

### Task 2: Update `assignParents` for namespace support

**Files:**
- Modify: `pkg/indexer/parser.go:27-74`
- Test: `pkg/indexer/csharp_test.go`

- [ ] **Step 1: Write failing test for namespace as parent of class**

Add to `pkg/indexer/csharp_test.go`:

```go
func TestGetSymbols_CSharp_Namespace_AsParent(t *testing.T) {
	const fixture = `
namespace DataAccess {
    public class UserRepository {
        public void Save() {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("repo.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UserRepository should have parent "DataAccess"
	for _, s := range symbols {
		if s.Name == "UserRepository" {
			if s.Parent != "DataAccess" {
				t.Errorf(`"UserRepository" parent: got %q, want %q`, s.Parent, "DataAccess")
			}
			return
		}
	}
	t.Fatal(`symbol "UserRepository" not found`)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestGetSymbols_CSharp_Namespace_AsParent -v`
Expected: FAIL — parent is `""` not `"DataAccess"`

- [ ] **Step 3: Update parentTypes, childTypes, and assignParents**

In `pkg/indexer/parser.go`, make these changes:

**a) Add Namespace to parentTypes (line ~27):**

```go
var parentTypes = map[SymbolType]bool{
	Class:     true,
	Interface: true,
	Enum:      true,
	Namespace: true,
}
```

**b) Add Class, Interface, Enum, Namespace to childTypes (line ~35):**

```go
var childTypes = map[SymbolType]bool{
	Method:    true,
	Variable:  true,
	Class:     true,
	Interface: true,
	Enum:      true,
	Namespace: true,
}
```

**c) Add symType to frame struct and FQN concatenation in assignParents (line ~48):**

```go
func assignParents(symbols []SymbolInfo) []SymbolInfo {
	type frame struct {
		name    string
		endLine int
		symType SymbolType
	}

	var stack []frame

	for i, s := range symbols {
		// Pop containers that have ended before this symbol starts.
		for len(stack) > 0 && s.StartLine > stack[len(stack)-1].endLine {
			stack = stack[:len(stack)-1]
		}

		// If this symbol is a child type and there is an open container, assign parent.
		if childTypes[s.Type] && len(stack) > 0 {
			symbols[i].Parent = stack[len(stack)-1].name
		}

		// If this symbol is itself a container, push it.
		if parentTypes[s.Type] {
			frameName := s.Name
			// FQN concatenation: namespace-on-namespace stacking.
			if s.Type == Namespace && len(stack) > 0 && stack[len(stack)-1].symType == Namespace {
				frameName = stack[len(stack)-1].name + "." + s.Name
			}
			stack = append(stack, frame{name: frameName, endLine: s.EndLine, symType: s.Type})
		}
	}

	return symbols
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestGetSymbols_CSharp_Namespace_AsParent -v`
Expected: PASS

- [ ] **Step 5: Run all existing tests to verify no regressions**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -v`
Expected: ALL PASS — existing behavior for methods/properties in classes is unchanged.

- [ ] **Step 6: Commit**

```bash
git add pkg/indexer/parser.go pkg/indexer/csharp_test.go
git commit -m "feat(csharp): extend assignParents to support namespaces as parent containers"
```

### Task 3: Test nested namespaces with FQN concatenation

**Files:**
- Test: `pkg/indexer/csharp_test.go`

- [ ] **Step 1: Write test for nested namespace FQN**

Add to `pkg/indexer/csharp_test.go`:

```go
func TestGetSymbols_CSharp_Namespace_NestedFQN(t *testing.T) {
	const fixture = `
namespace Company.Platform {
    namespace Services {
        public class OrderService {
            public void PlaceOrder() {}
        }
        public interface IPaymentGateway {
            void Charge();
        }
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("services.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build a map keyed by name+type for disambiguation
	type key struct{ name, typ string }
	byKey := make(map[key]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byKey[key{s.Name, string(s.Type)}] = s
	}

	cases := []struct {
		name       string
		typ        SymbolType
		wantParent string
	}{
		{"Company.Platform", Namespace, ""},
		{"Services", Namespace, "Company.Platform"},
		{"OrderService", Class, "Company.Platform.Services"},
		{"PlaceOrder", Method, "OrderService"},
		{"IPaymentGateway", Interface, "Company.Platform.Services"},
		{"Charge", Method, "IPaymentGateway"},
	}

	for _, tc := range cases {
		s, ok := byKey[key{tc.name, string(tc.typ)}]
		if !ok {
			t.Errorf("symbol %q (type %s) not found", tc.name, tc.typ)
			continue
		}
		if s.Parent != tc.wantParent {
			t.Errorf("%q parent: got %q, want %q", tc.name, s.Parent, tc.wantParent)
		}
	}
}
```

- [ ] **Step 2: Run test**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestGetSymbols_CSharp_Namespace_NestedFQN -v`
Expected: PASS (assignParents already handles this via FQN concatenation)

- [ ] **Step 3: Commit**

```bash
git add pkg/indexer/csharp_test.go
git commit -m "test(csharp): add nested namespace FQN concatenation test"
```

### Task 4: Test file-scoped namespace (C# 10+)

**Files:**
- Test: `pkg/indexer/csharp_test.go`

- [ ] **Step 1: Write test for file-scoped namespace**

Add to `pkg/indexer/csharp_test.go`:

```go
func TestGetSymbols_CSharp_Namespace_FileScoped(t *testing.T) {
	const fixture = `
namespace Company.Platform.Services;

public class NotificationService {
    public void Send(string message) {}
}

public enum Priority { Low, Medium, High }
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("notification.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type key struct{ name, typ string }
	byKey := make(map[key]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byKey[key{s.Name, string(s.Type)}] = s
	}

	cases := []struct {
		name       string
		typ        SymbolType
		wantParent string
	}{
		{"Company.Platform.Services", Namespace, ""},
		{"NotificationService", Class, "Company.Platform.Services"},
		{"Send", Method, "NotificationService"},
		{"Priority", Enum, "Company.Platform.Services"},
	}

	for _, tc := range cases {
		s, ok := byKey[key{tc.name, string(tc.typ)}]
		if !ok {
			t.Errorf("symbol %q (type %s) not found", tc.name, tc.typ)
			continue
		}
		if s.Parent != tc.wantParent {
			t.Errorf("%q parent: got %q, want %q", tc.name, s.Parent, tc.wantParent)
		}
	}
}
```

- [ ] **Step 2: Run test**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestGetSymbols_CSharp_Namespace_FileScoped -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add pkg/indexer/csharp_test.go
git commit -m "test(csharp): add file-scoped namespace test"
```

### Task 5: Test nested classes and multiple namespaces

**Files:**
- Test: `pkg/indexer/csharp_test.go`

- [ ] **Step 1: Write test for nested class inside namespace**

Add to `pkg/indexer/csharp_test.go`:

```go
func TestGetSymbols_CSharp_Namespace_NestedClass(t *testing.T) {
	const fixture = `
namespace Models {
    public class Order {
        public class LineItem {
            public decimal Price { get; set; }
        }
        public void Submit() {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("order.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type key struct{ name, typ string }
	byKey := make(map[key]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byKey[key{s.Name, string(s.Type)}] = s
	}

	cases := []struct {
		name       string
		typ        SymbolType
		wantParent string
	}{
		{"Models", Namespace, ""},
		{"Order", Class, "Models"},
		{"LineItem", Class, "Order"},
		{"Price", Variable, "LineItem"},
		{"Submit", Method, "Order"},
	}

	for _, tc := range cases {
		s, ok := byKey[key{tc.name, string(tc.typ)}]
		if !ok {
			t.Errorf("symbol %q (type %s) not found", tc.name, tc.typ)
			continue
		}
		if s.Parent != tc.wantParent {
			t.Errorf("%q parent: got %q, want %q", tc.name, s.Parent, tc.wantParent)
		}
	}
}
```

- [ ] **Step 2: Write test for multiple namespaces in one file**

Add to `pkg/indexer/csharp_test.go`:

```go
func TestGetSymbols_CSharp_Namespace_Multiple(t *testing.T) {
	const fixture = `
namespace Contracts {
    public interface ILogger {
        void Log(string msg);
    }
}

namespace Infrastructure {
    public class ConsoleLogger {
        public void Log(string msg) {}
    }
}
`
	m := newTestMuncher()
	symbols, err := m.GetSymbols("logging.cs", []byte(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type key struct{ name, typ string }
	byKey := make(map[key]SymbolInfo, len(symbols))
	for _, s := range symbols {
		byKey[key{s.Name, string(s.Type)}] = s
	}

	cases := []struct {
		name       string
		typ        SymbolType
		wantParent string
	}{
		{"Contracts", Namespace, ""},
		{"ILogger", Interface, "Contracts"},
		{"Infrastructure", Namespace, ""},
		{"ConsoleLogger", Class, "Infrastructure"},
	}

	for _, tc := range cases {
		s, ok := byKey[key{tc.name, string(tc.typ)}]
		if !ok {
			t.Errorf("symbol %q (type %s) not found", tc.name, tc.typ)
			continue
		}
		if s.Parent != tc.wantParent {
			t.Errorf("%q parent: got %q, want %q", tc.name, s.Parent, tc.wantParent)
		}
	}
}
```

- [ ] **Step 3: Run both tests**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run "TestGetSymbols_CSharp_Namespace_(NestedClass|Multiple)" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add pkg/indexer/csharp_test.go
git commit -m "test(csharp): add nested class and multiple namespace tests"
```

---

## Chunk 2: ParseDotNotation + Index Version Bump

### Task 6: Fix ParseDotNotation to split on last dot

**Files:**
- Modify: `pkg/indexer/lookup.go:51-75`
- Test: `pkg/indexer/lookup_test.go`

- [ ] **Step 1: Write failing test for FQN dot notation**

Add to `pkg/indexer/lookup_test.go`:

```go
func TestParseDotNotation_LastDotSplit(t *testing.T) {
	cases := []struct {
		input      SearchQuery
		wantParent string
		wantName   string
	}{
		// Single dot — unchanged behavior
		{SearchQuery{Name: "Server.serve"}, "Server", "serve"},
		// FQN — splits on last dot
		{SearchQuery{Name: "Company.Platform.Services.*"}, "Company.Platform.Services", ""},
		{SearchQuery{Name: "A.B.C"}, "A.B", "C"},
		// Wildcard parent — unchanged
		{SearchQuery{Name: "*.serve"}, "*", "serve"},
		// NameLike FQN
		{SearchQuery{NameLike: "Company.Platform.Ser"}, "Company.Platform", "Ser"},
	}

	for _, tc := range cases {
		got := ParseDotNotation(tc.input)
		if got.Parent != tc.wantParent {
			t.Errorf("ParseDotNotation(%+v) Parent: got %q, want %q", tc.input, got.Parent, tc.wantParent)
		}
		name := got.Name
		if name == "" {
			name = got.NameLike
		}
		if name != tc.wantName {
			t.Errorf("ParseDotNotation(%+v) Name: got %q, want %q", tc.input, name, tc.wantName)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestParseDotNotation_LastDotSplit -v`
Expected: FAIL — `"A.B.C"` gives parent=`A` instead of `A.B`

- [ ] **Step 3: Change IndexByte to LastIndexByte in ParseDotNotation**

In `pkg/indexer/lookup.go`, replace the `ParseDotNotation` function:

```go
func ParseDotNotation(q SearchQuery) SearchQuery {
	if dot := strings.LastIndexByte(q.Name, '.'); dot >= 0 {
		parent := q.Name[:dot]
		name := q.Name[dot+1:]
		q.Parent = parent
		if name == "*" {
			q.Name = ""
		} else {
			q.Name = name
		}
		return q
	}
	if dot := strings.LastIndexByte(q.NameLike, '.'); dot >= 0 {
		parent := q.NameLike[:dot]
		name := q.NameLike[dot+1:]
		q.Parent = parent
		if name == "*" {
			q.NameLike = ""
		} else {
			q.NameLike = name
		}
		return q
	}
	return q
}
```

- [ ] **Step 4: Run new test to verify it passes**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestParseDotNotation_LastDotSplit -v`
Expected: PASS

- [ ] **Step 5: Run all lookup tests to verify no regressions**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run TestSearchSymbols -v`
Expected: ALL PASS — single-dot behavior is unchanged.

- [ ] **Step 6: Commit**

```bash
git add pkg/indexer/lookup.go pkg/indexer/lookup_test.go
git commit -m "fix: ParseDotNotation splits on last dot for FQN namespace queries"
```

### Task 7: Bump indexVersion

**Files:**
- Modify: `pkg/indexer/types.go:6`

- [ ] **Step 1: Bump indexVersion from 5 to 6**

In `pkg/indexer/types.go`, change:

```go
const indexVersion = 6
```

- [ ] **Step 2: Run full test suite**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./... -v 2>&1 | tail -20`
Expected: ALL PASS (store_test.go uses `indexVersion` dynamically so it adapts)

- [ ] **Step 3: Commit**

```bash
git add pkg/indexer/types.go
git commit -m "chore: bump indexVersion to 6 for C# namespace indexing"
```

### Task 8: Full integration verification

- [ ] **Step 1: Run entire test suite**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./...`
Expected: ALL PASS, zero failures.

- [ ] **Step 2: Verify all C# namespace tests pass**

Run: `cd /Users/totolasso/repos/personal/mimir && go test ./pkg/indexer/ -run "TestGetSymbols_CSharp_Namespace" -v`
Expected: All 5 namespace tests pass:
- `TestGetSymbols_CSharp_Namespace_Simple`
- `TestGetSymbols_CSharp_Namespace_AsParent`
- `TestGetSymbols_CSharp_Namespace_NestedFQN`
- `TestGetSymbols_CSharp_Namespace_FileScoped`
- `TestGetSymbols_CSharp_Namespace_NestedClass`
- `TestGetSymbols_CSharp_Namespace_Multiple`
