package indexer

// refs_test.go — tests for cross-reference indexing:
//   call site extraction from Go source, SearchRefs by callee/caller/file,
//   atomic re-index (old refs replaced on re-write), and empty-result case.

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

// goCallFixture is a small Go snippet with three distinct call sites.
// foo calls bar and baz; bar calls baz.
const goCallFixture = `package main

func baz() {}

func bar() {
	baz()
}

func foo() {
	bar()
	baz()
}
`

// --- call-site extraction ---

func TestExtractCalls_GoFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("go", []byte(goCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls: %v", err)
	}

	// foo calls bar (line 10) and baz (line 11); bar calls baz (line 6).
	if len(calls) < 3 {
		t.Fatalf("expected at least 3 call sites, got %d: %v", len(calls), calls)
	}

	// Collect callee names into a set.
	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, want := range []string{"bar", "baz"} {
		if !names[want] {
			t.Errorf("expected callee %q in extracted calls, got %v", want, calls)
		}
	}
}

func TestExtractCalls_UnsupportedLanguageReturnsEmpty(t *testing.T) {
	calls, err := ExtractCalls("ruby", []byte("puts 'hello'"))
	if err != nil {
		t.Fatalf("ExtractCalls on unsupported lang should not error: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 calls for unsupported language, got %d", len(calls))
	}
}

// --- SearchRefs ---

// seedRefsDB writes a small two-file fixture into an in-memory test DB.
//
//	caller.go — foo calls bar (line 10) and baz (line 11); bar calls baz (line 6)
//	other.go  — run calls foo (line 3)
func seedRefsDB(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestDB(t, t.TempDir())

	callerEntry := FileEntry{
		Language:  "go",
		SHA256:    "hash1",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "baz", Type: Function, StartLine: 3, EndLine: 3},
			{Name: "bar", Type: Function, StartLine: 5, EndLine: 7},
			{Name: "foo", Type: Function, StartLine: 9, EndLine: 12},
		},
		Calls: []CallSite{
			{CalleeName: "baz", Line: 6},
			{CalleeName: "bar", Line: 10},
			{CalleeName: "baz", Line: 11},
		},
	}

	otherEntry := FileEntry{
		Language:  "go",
		SHA256:    "hash2",
		IndexedAt: time.Now().UTC(),
		Symbols: []SymbolInfo{
			{Name: "run", Type: Function, StartLine: 1, EndLine: 5},
		},
		Calls: []CallSite{
			{CalleeName: "foo", Line: 3},
		},
	}

	if err := WriteFile(db, "caller.go", callerEntry); err != nil {
		t.Fatalf("WriteFile caller.go: %v", err)
	}
	if err := WriteFile(db, "other.go", otherEntry); err != nil {
		t.Fatalf("WriteFile other.go: %v", err)
	}

	return db
}

func TestSearchRefs_ByCallee(t *testing.T) {
	db := seedRefsDB(t)

	rows, err := SearchRefs(db, RefQuery{CalleeName: "baz"})
	if err != nil {
		t.Fatalf("SearchRefs by callee: %v", err)
	}

	// baz is called twice in caller.go (lines 6 and 11).
	if len(rows) != 2 {
		t.Fatalf("expected 2 refs to baz, got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r.CalleeName != "baz" {
			t.Errorf("expected callee_name=baz, got %q", r.CalleeName)
		}
		if r.CallerFile != "caller.go" {
			t.Errorf("expected caller_file=caller.go, got %q", r.CallerFile)
		}
	}
}

func TestSearchRefs_ByCallerName(t *testing.T) {
	db := seedRefsDB(t)

	rows, err := SearchRefs(db, RefQuery{CallerName: "foo"})
	if err != nil {
		t.Fatalf("SearchRefs by caller name: %v", err)
	}

	// foo calls bar and baz.
	if len(rows) != 2 {
		t.Fatalf("expected 2 refs from foo, got %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		if r.CallerName != "foo" {
			t.Errorf("expected caller_name=foo, got %q", r.CallerName)
		}
	}
}

func TestSearchRefs_ByCallerFile(t *testing.T) {
	db := seedRefsDB(t)

	rows, err := SearchRefs(db, RefQuery{CallerFile: "other.go"})
	if err != nil {
		t.Fatalf("SearchRefs by caller file: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 ref from other.go, got %d: %v", len(rows), rows)
	}
	if rows[0].CalleeName != "foo" {
		t.Errorf("expected callee_name=foo, got %q", rows[0].CalleeName)
	}
}

func TestSearchRefs_EmptyResult(t *testing.T) {
	db := seedRefsDB(t)

	rows, err := SearchRefs(db, RefQuery{CalleeName: "doesNotExist"})
	if err != nil {
		t.Fatalf("SearchRefs empty: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for unknown callee, got %d", len(rows))
	}
}

// TestSearchRefs_EmptyResult_JSONNotNull ensures that an empty result set
// marshals to a JSON array ("[]") rather than JSON null.
// Regression: SearchRefs previously returned a nil slice which json.Marshal
// encodes as null, breaking callers that expect an array.
func TestSearchRefs_EmptyResult_JSONNotNull(t *testing.T) {
	db := seedRefsDB(t)

	rows, err := SearchRefs(db, RefQuery{CalleeName: "doesNotExist"})
	if err != nil {
		t.Fatalf("SearchRefs empty: %v", err)
	}

	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if string(data) == "null" {
		t.Error("SearchRefs returned a nil slice: JSON output is null, want []")
	}
	if string(data) != "[]" {
		t.Errorf("expected JSON [], got %s", string(data))
	}
}

func TestSearchRefs_NoFilter_ReturnsAll(t *testing.T) {
	db := seedRefsDB(t)

	// 3 calls in caller.go + 1 in other.go = 4 total.
	rows, err := SearchRefs(db, RefQuery{})
	if err != nil {
		t.Fatalf("SearchRefs no filter: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 total refs, got %d: %v", len(rows), rows)
	}
}

// --- atomic re-index: old refs replaced on WriteFile ---

func TestWriteFile_ReplacesOldRefs(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	first := FileEntry{
		Language:  "go",
		SHA256:    "v1",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "foo", Type: Function, StartLine: 1, EndLine: 5}},
		Calls:     []CallSite{{CalleeName: "oldFunc", Line: 3}},
	}
	if err := WriteFile(db, "file.go", first); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}

	second := FileEntry{
		Language:  "go",
		SHA256:    "v2",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "foo", Type: Function, StartLine: 1, EndLine: 5}},
		Calls:     []CallSite{{CalleeName: "newFunc", Line: 3}},
	}
	if err := WriteFile(db, "file.go", second); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	rows, err := SearchRefs(db, RefQuery{CallerFile: "file.go"})
	if err != nil {
		t.Fatalf("SearchRefs: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 ref after re-write, got %d: %v", len(rows), rows)
	}
	if rows[0].CalleeName != "newFunc" {
		t.Errorf("expected callee_name=newFunc, got %q", rows[0].CalleeName)
	}
}

// --- PruneFiles cascades to refs ---

func TestPruneFiles_CascadesToRefs(t *testing.T) {
	db := openTestDB(t, t.TempDir())

	entry := FileEntry{
		Language:  "go",
		SHA256:    "x",
		IndexedAt: time.Now().UTC(),
		Symbols:   []SymbolInfo{{Name: "foo", Type: Function, StartLine: 1, EndLine: 5}},
		Calls:     []CallSite{{CalleeName: "bar", Line: 3}},
	}
	if err := WriteFile(db, "foo.go", entry); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := PruneFiles(db, []string{"foo.go"}); err != nil {
		t.Fatalf("PruneFiles: %v", err)
	}

	rows, err := SearchRefs(db, RefQuery{CallerFile: "foo.go"})
	if err != nil {
		t.Fatalf("SearchRefs after prune: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 refs after prune, got %d", len(rows))
	}
}

// --- ref query: identifier references used as values ---

// goRefFixture is a Go snippet where runA is passed as a struct field value
// and runB is assigned to a variable — neither is called directly.
const goRefFixture = `package main

import "github.com/spf13/cobra"

func runA(cmd *cobra.Command, args []string) error { return nil }
func runB(cmd *cobra.Command, args []string) error { return nil }
func runC(cmd *cobra.Command, args []string) error { return nil }

var cmdA = &cobra.Command{RunE: runA}
var f = runB
`

func TestExtractCalls_GoCaptures_StructFieldRef(t *testing.T) {
	calls, err := ExtractCalls("go", []byte(goRefFixture))
	if err != nil {
		t.Fatalf("ExtractCalls: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["runA"] {
		t.Error("runA is used as RunE field value and must appear in extracted refs")
	}
	if !names["runB"] {
		t.Error("runB is assigned to a var and must appear in extracted refs")
	}
	// runC is declared but never referenced — should not appear.
	if names["runC"] {
		t.Error("runC is never referenced and must not appear in extracted refs")
	}
}

func TestExtractCalls_GoCaptures_ShortVarAndAssignment(t *testing.T) {
	code := []byte(`package main

func handler() {}
func other() {}

func setup() {
	f := handler
	f = other
}
`)
	calls, err := ExtractCalls("go", []byte(code))
	if err != nil {
		t.Fatalf("ExtractCalls: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["handler"] {
		t.Error("handler is used in short var decl and must appear in extracted refs")
	}
	if !names["other"] {
		t.Error("other is used in assignment and must appear in extracted refs")
	}
}

// --- JavaScript call + ref extraction ---

// jsCallFixture has plain calls, method calls, and a function passed as a value.
const jsCallFixture = `
function bar() {}
function baz() {}
function runA() {}
function runB() {}
function unused() {}

function foo() {
  bar();
  obj.baz();
}

const cmd = { handler: runA };
const f = runB;
`

func TestExtractCalls_JSFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, want := range []string{"bar", "baz"} {
		if !names[want] {
			t.Errorf("expected callee %q in JS extracted calls, got %v", want, calls)
		}
	}
}

func TestExtractCalls_JSCaptures_ObjectFieldRef(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["runA"] {
		t.Error("runA is used as object field value and must appear in extracted refs")
	}
	if !names["runB"] {
		t.Error("runB is assigned to a const and must appear in extracted refs")
	}
	if names["unused"] {
		t.Error("unused is never referenced and must not appear in extracted refs")
	}
}

// --- TypeScript call + ref extraction ---

// tsCallFixture has plain calls and a function passed as a value in TS.
const tsCallFixture = `
function bar(): void {}
function baz(): void {}
function runA(): void {}
function runB(): void {}
function unused(): void {}

function foo(): void {
  bar();
  obj.baz();
}

const cmd = { handler: runA };
const f = runB;
`

func TestExtractCalls_TSFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("typescript", []byte(tsCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TS: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, want := range []string{"bar", "baz"} {
		if !names[want] {
			t.Errorf("expected callee %q in TS extracted calls, got %v", want, calls)
		}
	}
}

func TestExtractCalls_TSCaptures_ObjectFieldRef(t *testing.T) {
	calls, err := ExtractCalls("typescript", []byte(tsCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TS: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["runA"] {
		t.Error("runA is used as object field value and must appear in extracted refs")
	}
	if !names["runB"] {
		t.Error("runB is assigned to a const and must appear in extracted refs")
	}
	if names["unused"] {
		t.Error("unused is never referenced and must not appear in extracted refs")
	}
}

// TSX shares the TypeScript grammar — verify it is also registered.
func TestExtractCalls_TSXFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("tsx", []byte(tsCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, want := range []string{"bar", "baz"} {
		if !names[want] {
			t.Errorf("expected callee %q in TSX extracted calls, got %v", want, calls)
		}
	}
}

// --- TSX JSX call + ref extraction ---

// tsxJSXCallFixture contains JSX opening elements, self-closing elements, and
// native HTML tags. Only PascalCase/uppercase-initial names should be captured.
const tsxJSXCallFixture = `
import React from 'react';

function MeetingPointComponent() { return null; }
function AddressBookForm() { return null; }
function unused() { return null; }

function Page() {
  return (
    <div>
      <MeetingPointComponent key="mp" />
      <AddressBookForm onSubmit={handleSubmit}>
        <span>child</span>
      </AddressBookForm>
    </div>
  );
}
`

func TestExtractCalls_TSX_JSXSelfClosingElement(t *testing.T) {
	calls, err := ExtractCalls("tsx", []byte(tsxJSXCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TSX JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["MeetingPointComponent"] {
		t.Error("MeetingPointComponent is used as a self-closing JSX element and must appear in extracted refs")
	}
}

func TestExtractCalls_TSX_JSXOpeningElement(t *testing.T) {
	calls, err := ExtractCalls("tsx", []byte(tsxJSXCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TSX JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["AddressBookForm"] {
		t.Error("AddressBookForm is used as a JSX opening element and must appear in extracted refs")
	}
}

func TestExtractCalls_TSX_NativeHTMLTagsNotCaptured(t *testing.T) {
	calls, err := ExtractCalls("tsx", []byte(tsxJSXCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TSX JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, tag := range []string{"div", "span", "input", "button", "p", "h1"} {
		if names[tag] {
			t.Errorf("native HTML tag %q must NOT appear in extracted refs", tag)
		}
	}
}

func TestExtractCalls_TSX_UnusedComponentNotCaptured(t *testing.T) {
	calls, err := ExtractCalls("tsx", []byte(tsxJSXCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TSX JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if names["unused"] {
		t.Error("unused is never referenced as a JSX element or called and must not appear in extracted refs")
	}
}

// tsxJSXLineFixture is used to verify that the line number of a JSX ref points
// to the tag's opening line, not the component definition line.
const tsxJSXLineFixture = `import React from 'react';

function Widget() { return null; }

function App() {
  return <Widget />;
}
`

func TestExtractCalls_TSX_JSXRefLineNumber(t *testing.T) {
	calls, err := ExtractCalls("tsx", []byte(tsxJSXLineFixture))
	if err != nil {
		t.Fatalf("ExtractCalls TSX JSX line: %v", err)
	}

	for _, c := range calls {
		if c.CalleeName == "Widget" {
			// <Widget /> appears on line 6.
			if c.Line != 6 {
				t.Errorf("Widget JSX ref: got line %d, want 6", c.Line)
			}
			return
		}
	}
	t.Error("Widget JSX ref not found in extracted calls")
}

// --- JSX in plain JavaScript (.jsx) ---

// jsxComponentFixture exercises JSX extraction for the JavaScript grammar
// (used for .jsx files). It mirrors tsxJSXCallFixture so the same set of
// assertions can be made against both grammars.
//
// Layout:
//
//	line 1  – import
//	line 3  – NavBar declaration (self-closing usage on line 11)
//	line 4  – SideBar declaration (opening-element usage on line 12)
//	line 5  – lowercaseComp declaration (must NOT be captured)
//	line 7  – Page function
//	line 9  – <div> native tag (must NOT be captured)
//	line 10 – <section> native tag (must NOT be captured)
//	line 11 – <NavBar /> self-closing
//	line 12 – <SideBar …> opening element
//	line 13 –   <span> nested native tag (must NOT be captured)
const jsxComponentFixture = `import React from 'react';

function NavBar() { return null; }
function SideBar() { return null; }
function lowercaseComp() { return null; }

function Page() {
  return (
    <div>
      <section>
        <NavBar />
        <SideBar className="side">
          <span>child</span>
        </SideBar>
      </section>
    </div>
  );
}
`

// jsxLineFixture is used to verify that the line number of a JSX ref in a
// .jsx file points to the tag's usage line, not the component definition line.
//
// Layout:
//
//	line 1  – import
//	line 3  – Logo declaration
//	line 5  – App function
//	line 6  – return <Logo />;
const jsxLineFixture = `import React from 'react';

function Logo() { return null; }

function App() {
  return <Logo />;
}
`

func TestExtractCalls_JSX_SelfClosingElement(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsxComponentFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["NavBar"] {
		t.Error("NavBar is used as a self-closing JSX element and must appear in extracted refs for .jsx files")
	}
}

func TestExtractCalls_JSX_OpeningElement(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsxComponentFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["SideBar"] {
		t.Error("SideBar is used as a JSX opening element and must appear in extracted refs for .jsx files")
	}
}

func TestExtractCalls_JSX_NativeHTMLTagsNotCaptured(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsxComponentFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, tag := range []string{"div", "section", "span", "input", "button", "p", "h1"} {
		if names[tag] {
			t.Errorf("native HTML tag %q must NOT appear in extracted refs for .jsx files", tag)
		}
	}
}

func TestExtractCalls_JSX_UnusedComponentNotCaptured(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsxComponentFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS JSX: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if names["lowercaseComp"] {
		t.Error("lowercaseComp is never referenced as a JSX element or called and must not appear in extracted refs")
	}
}

func TestExtractCalls_JSX_RefLineNumber(t *testing.T) {
	calls, err := ExtractCalls("javascript", []byte(jsxLineFixture))
	if err != nil {
		t.Fatalf("ExtractCalls JS JSX line: %v", err)
	}

	for _, c := range calls {
		if c.CalleeName == "Logo" {
			// <Logo /> appears on line 6.
			if c.Line != 6 {
				t.Errorf("Logo JSX ref: got line %d, want 6", c.Line)
			}
			return
		}
	}
	t.Error("Logo JSX ref not found in extracted calls for .jsx file")
}

// pyCallFixture has plain calls, method calls, and a function assigned to a variable.
const pyCallFixture = `
def bar():
    pass

def baz():
    pass

def run_a():
    pass

def run_b():
    pass

def unused():
    pass

def foo():
    bar()
    obj.baz()

cmd = {"handler": run_a}
f = run_b
`

func TestExtractCalls_PythonFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("python", []byte(pyCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls Python: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, want := range []string{"bar", "baz"} {
		if !names[want] {
			t.Errorf("expected callee %q in Python extracted calls, got %v", want, calls)
		}
	}
}

func TestExtractCalls_PythonCaptures_AssignmentRef(t *testing.T) {
	calls, err := ExtractCalls("python", []byte(pyCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls Python: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["run_a"] {
		t.Error("run_a is used as dict value and must appear in extracted refs")
	}
	if !names["run_b"] {
		t.Error("run_b is assigned to a variable and must appear in extracted refs")
	}
	if names["unused"] {
		t.Error("unused is never referenced and must not appear in extracted refs")
	}
}

// --- C# call + ref extraction ---

// csCallFixture has plain method calls and a delegate/action assignment.
const csCallFixture = `
using System;

class App {
    void Bar() {}
    void Baz() {}
    void RunA() {}
    void RunB() {}
    void Unused() {}

    void Foo() {
        Bar();
        obj.Baz();
    }

    void Setup() {
        Action a = RunA;
        Action b = RunB;
    }
}
`

func TestExtractCalls_CSharpFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("csharp", []byte(csCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls C#: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	for _, want := range []string{"Bar", "Baz"} {
		if !names[want] {
			t.Errorf("expected callee %q in C# extracted calls, got %v", want, calls)
		}
	}
}

func TestExtractCalls_CSharpCaptures_DelegateAssignmentRef(t *testing.T) {
	calls, err := ExtractCalls("csharp", []byte(csCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls C#: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	if !names["RunA"] {
		t.Error("RunA is assigned to a delegate and must appear in extracted refs")
	}
	if !names["RunB"] {
		t.Error("RunB is assigned to a delegate and must appear in extracted refs")
	}
	if names["Unused"] {
		t.Error("Unused is never referenced and must not appear in extracted refs")
	}
}

// ---------------------------------------------------------------------------
// Rust
// ---------------------------------------------------------------------------

const rustCallFixture = `
fn helper() {}

fn plain_call() {
    helper();
}

struct Server {
    port: u16,
}

impl Server {
    fn new(port: u16) -> Self { Server { port } }

    fn start(&self) {
        self.listen();
    }

    fn listen(&self) {}
}

fn use_server() {
    let s = Server::new(8080);
    s.start();
    println!("running");
    vec![1, 2, 3];
}

fn assigned_ref() {
    let f = helper;
    let g = plain_call;
}
`

func TestExtractCalls_RustFindsCallSites(t *testing.T) {
	calls, err := ExtractCalls("rust", []byte(rustCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls Rust: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	// Plain call
	if !names["helper"] {
		t.Error("expected plain call to 'helper'")
	}
	// Method call (self.listen)
	if !names["listen"] {
		t.Error("expected method call to 'listen'")
	}
	// Associated function (Server::new)
	if !names["new"] {
		t.Error("expected associated function call 'new'")
	}
	// Method call on variable (s.start)
	if !names["start"] {
		t.Error("expected method call to 'start'")
	}
	// Macro invocations
	if !names["println"] {
		t.Error("expected macro invocation 'println'")
	}
	if !names["vec"] {
		t.Error("expected macro invocation 'vec'")
	}
}

func TestExtractCalls_RustCaptures_LetBindingRef(t *testing.T) {
	calls, err := ExtractCalls("rust", []byte(rustCallFixture))
	if err != nil {
		t.Fatalf("ExtractCalls Rust: %v", err)
	}

	names := make(map[string]bool, len(calls))
	for _, c := range calls {
		names[c.CalleeName] = true
	}

	// let f = helper; should capture 'helper' as a ref
	if !names["helper"] {
		t.Error("helper assigned via let binding must appear in extracted refs")
	}
	// let g = plain_call; should capture 'plain_call' as a ref
	if !names["plain_call"] {
		t.Error("plain_call assigned via let binding must appear in extracted refs")
	}
}
