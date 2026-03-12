# Mimir Backlog

## #6 — Complexity Metrics

**Status**: Backlog

**Summary**: Compute and store cyclomatic complexity for every function/method
at parse time (using the tree-sitter AST), making it queryable and filterable.

**What it enables**:
- `mimir symbols --min-complexity 10 <root>` — find hot spots
- Complexity trend tracking across re-indexes

**Scope**:
- Schema: add `complexity INT DEFAULT 0` column to `symbols`
- Parser: after extracting a symbol node, walk its subtree counting branch
  nodes (`if`, `for`, `switch`, `case`, `&&`, `||`) — cyclomatic = branches + 1
- Per language: define branch node-type tables for Go, JS, TS, Python, C#
- Store: `WriteFile` batch INSERT already handles extra symbol fields
- Lookup: `SearchSymbols` / `SearchQuery` gains `MinComplexity int` filter
- CLI: `--min-complexity` flag on `mimir search`
- Tests: complexity calculation verified per language

**Estimated effort**: ~1–2 days
