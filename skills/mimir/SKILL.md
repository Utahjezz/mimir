---
name: mimir
description: "Tree-sitter code indexer for exploring symbols, tracing call graphs, querying imports, and detecting dead code. Use this skill whenever you need to understand a codebase structure, find where a function is defined, trace who calls what, inspect what a file imports, find who imports a module, analyze module boundaries, detect unused code, or get a high-level overview of a repository. Trigger on requests like 'index this repo', 'find symbol X', 'who calls this function', 'what does this file import?', 'who imports this package?', 'show dead code', 'trace the call graph', 'explore this codebase', 'show repo structure', or before refactoring to understand impact. Also use it during cross-repo exploration and proactively register high-confidence workspace links you discover."
version: 1.4.0
type: skill
category: development
tags:
  - indexer
  - code-exploration
  - tree-sitter
  - sqlite
  - symbols
  - call-graph
  - dead-code
  - cli
user-invocable: true
argument-hint: "[command or exploration goal]"
allowed-tools: Bash, Read, Glob, Grep
metadata:
  filePattern:
    - "**/*.py"
    - "**/*.ts"
    - "**/*.tsx"
    - "**/*.js"
    - "**/*.go"
    - "**/*.cs"
    - "**/*.rs"
    - "**/*.swift"
  bashPattern:
    - "mimir.*"
---

# Mimir — Code Indexer & Explorer

## Overview

Index a repo once, then query symbols, imports, call graphs, fuzzy matches, and dead code from a persistent SQLite index built by tree-sitter.

## Operating Mode

This skill is **capability-first, not interface-specific**.

- Use whatever Mimir interface is available in the current environment
- If structured Mimir tools are available, use them
- If they are not available, use the CLI commands documented here
- Do not assume a particular tool integration is installed before you start

Translate the user's exploration goal into the most direct Mimir operation available.

## Quick Start

Always index before querying:

```bash
mimir index <path>                    # Index the repo (incremental)
mimir report <path>                   # Overview: files, symbols, languages
mimir tree <path> --depth 3           # Directory structure with symbol counts
```

## When to Use Each Command

| Goal | Command | Notes |
|------|---------|-------|
| **First-time orientation** | `mimir index` then `mimir report` then `mimir tree --depth 3` | Always start here on a new repo |
| **Find a symbol definition** | `mimir symbol <root> <name>` | Prints full source. Use `--type` to disambiguate |
| **Search symbols by pattern** | `mimir search <root> --fuzzy "query"` | camelCase/snake_case aware, BM25 ranked, searches names + body; add `--limit N` to cap results |
| **Exact name lookup** | `mimir search <root> --name "ClassName.method"` | Dot-notation: `Class.*`, `*.method` |
| **Prefix search** | `mimir search <root> --like "process"` | SQL LIKE prefix match |
| **What does this file import?** | `mimir imports <root> --file <path>` | Lists import statements for one indexed source file |
| **Who imports this module/package?** | `mimir imports <root> --module <path>` | Great for module boundary checks and rename impact |
| **Who calls this function?** | `mimir callers <root> <symbol>` | Default 2 levels deep. Use `--depth N` |
| **What does this function call?** | `mimir refs <root> --caller <name>` | Outbound references |
| **Simulate refactor impact** | `mimir impact simulate <root> --symbol <name> --change <descriptor>` | Returns risk + planning signals before editing |
| **Most-called symbols (hotspots)** | `mimir refs <root> --hotspot` | Great for finding load-bearing code |
| **Dead code detection** | `mimir dead <root> --unexported` | `--unexported` reduces false positives |
| **Quick file inspection** | `mimir symbols <file>` | No index needed — parses on the fly |

**Not the right tool when:** you need full-text search across file *contents* (not symbol names/bodies) — use `grep`/`rg` for that. Mimir indexes symbols and call references, not raw line text.

## Supported Languages

`.py` `.pyw` `.js` `.jsx` `.mjs` `.cjs` `.ts` `.mts` `.cts` `.tsx` `.go` `.cs` `.rs` `.swift`

All other file types are silently skipped. Dot-directories (`.git`, `.venv`), `node_modules`, and `vendor` are always skipped.

## Key Behaviors

- **Incremental indexing** — only changed files are re-parsed (mtime+size check)
- **Auto-refresh** — query commands auto-reindex if index is >10s stale. Use `--no-refresh` to skip
- **JSON output** — all commands support `--json` for piping to `jq`
- **Schema versioned** — if schema changed, run `mimir index --rebuild <path>`

## Recommended Workflows

### Orientation in an unfamiliar repo
```bash
mimir index /path/to/repo
mimir report /path/to/repo
mimir tree /path/to/repo --files --depth 3
mimir refs /path/to/repo --hotspot --limit 10    # find the important symbols
```

### Before refactoring a function
```bash
mimir impact simulate /path/to/repo --symbol MyFunction --change "rename_symbol:to=MyFunctionV2" --json
mimir callers /path/to/repo MyFunction            # who depends on this?
mimir callers /path/to/repo MyFunction --depth 3  # deeper impact analysis
mimir refs /path/to/repo --caller MyFunction      # what does it call?
```

### Import and dependency analysis
```bash
mimir imports /path/to/repo --file pkg/orders/service.go
mimir imports /path/to/repo --module github.com/acme/payments/sdk
mimir imports --workspace myproject --module github.com/acme/shared/contracts
```

Use this when the question is about package/module dependencies rather than symbol definitions.

### Agent-aware planning before code edits
```bash
mimir impact simulate /path/to/repo \
  --symbol CheckoutService.process \
  --change "add_required_param:param_name=currency:param_type_to=string" \
  --json
```

Use the JSON output to adapt planning automatically:
- high risk score → split migration / adapter strategy
- cross-repo boundaries → coordinate consumers + contract tests
- low confidence → require manual review step before implementation

### Finding and understanding a symbol
```bash
mimir symbol /path/to/repo GetSymbols             # full source code
mimir search /path/to/repo --fuzzy "order place"  # fuzzy search in names + body
mimir search /path/to/repo --name "Order.*"       # all methods on Order class
```

### Dead code audit
```bash
mimir dead /path/to/repo --unexported             # unexported = fewer false positives
mimir dead /path/to/repo --type function --file pkg/utils/
```

### Cross-repo exploration (workspaces)
See `references/workspaces.md` for workspace commands (create, link, fan-out search).

## Cross-Repo Link Registration Policy

```
AFTER CROSS-REPO EXPLORATION: REGISTER HIGH-CONFIDENCE WORKSPACE LINKS BY DEFAULT
```

**Apply this policy only when:** a workspace exists and `mimir workspace show` returns at least 2 repos. If no workspace exists or it contains fewer than 2 repos, skip silently.

When workspace exploration reveals a credible cross-repo relationship, treat link registration as a normal part of the workflow — not an optional follow-up.

**Autonomously register the link when all are true:**
- The source and destination symbols resolve clearly
- The relationship is plausible from refs, naming, or surrounding code context
- The link is not already declared
- You can describe the relationship with a concise, useful note

**Before creating a link:**
1. Review existing links to avoid duplicates
2. Confirm source/destination direction
3. Add a meaningful `--note` and at least one relevant `--meta` value when possible
4. Run `mimir workspace links --check` after registration when validation is useful

**Ask the user only when:**
- Multiple symbol matches make the link ambiguous
- The relationship is speculative or weakly inferred
- You cannot determine the correct source/destination symbols confidently
- The intended note/metadata would be mostly guesswork

**Never end cross-repo exploration without doing one of these:**
- registering the high-confidence links you found, or
- explicitly reporting the ambiguous candidates you chose not to register

**Final summary requirement:** report every link you created, why it was added, and any candidate links left unresolved.

For the concrete step-by-step flow, use `references/workspaces.md` → **Link Discovery Protocol**.

## Important Caveats

1. **Always index first** — all query commands need an existing index (except `symbols` and `symbol` in file mode)
2. **Dead-code uses name-only matching** — false negatives possible for common names like `Open`, `Close`, `Error`. Use `--unexported` to reduce noise
3. **Framework entry points show as "dead"** — route handlers, decorators, fixtures are called by frameworks, not directly in code. These are expected false positives
4. **Imports are path-oriented** — use `imports` for module/package dependency questions, not `search`

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Querying before indexing | Always run `mimir index <path>` first; `mimir symbols <file>` is the only command that works without an index |
| Using `--name` for approximate matches | `--name` is exact. Use `--fuzzy` for partial/camelCase matches |
| Using `search` when the task is really about imports | Use `mimir imports --file` or `--module` for dependency/module questions |
| Treating dead-code results as definitive | `mimir dead` uses name-only matching — common names (`Open`, `Close`, `Error`) produce false negatives. Always review results manually |
| Forgetting `--unexported` on dead-code runs | Without it, every exported symbol shows as "dead" even if called by external packages |
| Skipping link declaration after workspace exploration | Cross-repo relationships found this session are gone next session if not declared with `mimir workspace link` |

## Full Command Reference

For complete flag documentation, output formats, and examples:
- **All commands**: Read `references/commands.md`
- **Workspace commands**: Read `references/workspaces.md`
