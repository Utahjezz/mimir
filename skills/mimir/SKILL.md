---
name: mimir
description: "Tree-sitter code indexer for exploring symbols, tracing call graphs, and detecting dead code. Use this skill whenever you need to understand a codebase structure, find where a function is defined, trace who calls what, search for symbols by name or pattern, detect unused code, or get a high-level overview of a repository. Trigger on: 'index this repo', 'find symbol X', 'who calls this function', 'show dead code', 'trace the call graph', 'explore this codebase', 'what symbols are in this file', 'show repo structure', or any codebase exploration task. Also use when navigating unfamiliar repos or before refactoring to understand impact."
version: 1.2.0
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
  bashPattern:
    - "mimir.*"
---

# Mimir — Code Indexer & Explorer

## Overview

Index a repo once, then query symbols, trace call graphs, search by name or fuzzy text, and detect dead code — all from a persistent SQLite index built by tree-sitter.

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
| **Who calls this function?** | `mimir callers <root> <symbol>` | Default 2 levels deep. Use `--depth N` |
| **What does this function call?** | `mimir refs <root> --caller <name>` | Outbound references |
| **Most-called symbols (hotspots)** | `mimir refs <root> --hotspot` | Great for finding load-bearing code |
| **Dead code detection** | `mimir dead <root> --unexported` | `--unexported` reduces false positives |
| **Quick file inspection** | `mimir symbols <file>` | No index needed — parses on the fly |

**Not the right tool when:** you need full-text search across file *contents* (not symbol names/bodies) — use `grep`/`rg` for that. Mimir indexes symbols and call references, not raw line text.

## Supported Languages

`.py` `.pyw` `.js` `.jsx` `.mjs` `.cjs` `.ts` `.mts` `.cts` `.tsx` `.go` `.cs`

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
mimir callers /path/to/repo MyFunction            # who depends on this?
mimir callers /path/to/repo MyFunction --depth 3  # deeper impact analysis
mimir refs /path/to/repo --caller MyFunction      # what does it call?
```

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

## Cross-Repo Link Obligation

```
AFTER EXPLORING ANY WORKSPACE: ASK THE USER BEFORE RUNNING THE LINK DISCOVERY PROTOCOL
```

**Only ask if:** a workspace exists and `mimir workspace show` returns at least 2 repos. If no workspace exists or it contains fewer than 2 repos, skip silently.

When the conditions above are met, ask the user before proceeding:

> "I can run the Link Discovery Protocol to find and declare cross-repo relationships
> discovered during this session. This inspects outbound refs across all workspace repos.
> Proceed?"

**Ask when:**
- You explored two or more repos in a workspace
- You traced a symbol from one repo that resolves in another
- You found naming patterns suggesting a caller/callee relationship across repos

**If the user confirms:** run the protocol in `references/workspaces.md` → **Link Discovery Protocol**.

**If the user declines:** briefly note any obvious candidates you already observed, so
they can run the protocol themselves later. Do not silently discard what you found.

**Red Flags — STOP and ask:**
- "The relationship is obvious from context" — undeclared links don't exist in the index
- "I only explored briefly" — even a brief exploration can surface a link worth declaring
- Ending the session without having asked — there is no next time, future sessions start blind

## Important Caveats

1. **Always index first** — all query commands need an existing index (except `symbols` and `symbol` in file mode)
2. **Dead-code uses name-only matching** — false negatives possible for common names like `Open`, `Close`, `Error`. Use `--unexported` to reduce noise
3. **Framework entry points show as "dead"** — route handlers, decorators, fixtures are called by frameworks, not directly in code. These are expected false positives

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Querying before indexing | Always run `mimir index <path>` first; `mimir symbols <file>` is the only command that works without an index |
| Using `--name` for approximate matches | `--name` is exact. Use `--fuzzy` for partial/camelCase matches |
| Treating dead-code results as definitive | `mimir dead` uses name-only matching — common names (`Open`, `Close`, `Error`) produce false negatives. Always review results manually |
| Forgetting `--unexported` on dead-code runs | Without it, every exported symbol shows as "dead" even if called by external packages |
| Skipping link declaration after workspace exploration | Cross-repo relationships found this session are gone next session if not declared with `mimir workspace link` |

## Full Command Reference

For complete flag documentation, output formats, and examples:
- **All commands**: Read `references/commands.md`
- **Workspace commands**: Read `references/workspaces.md`
