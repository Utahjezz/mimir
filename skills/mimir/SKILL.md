---
name: mimir
description: "Tree-sitter code indexer for exploring symbols, tracing call graphs, and detecting dead code. Use this skill whenever you need to understand a codebase structure, find where a function is defined, trace who calls what, search for symbols by name or pattern, detect unused code, or get a high-level overview of a repository. Trigger on: 'index this repo', 'find symbol X', 'who calls this function', 'show dead code', 'trace the call graph', 'explore this codebase', 'what symbols are in this file', 'show repo structure', or any codebase exploration task. Also use when navigating unfamiliar repos or before refactoring to understand impact."
version: 1.1.0
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

> Index source code with tree-sitter, persist to SQLite, then explore symbols, search, trace references, and detect dead code from the CLI.

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
| **Search symbols by pattern** | `mimir search <root> --fuzzy "query"` | camelCase/snake_case aware, searches names + body |
| **Exact name lookup** | `mimir search <root> --name "ClassName.method"` | Dot-notation: `Class.*`, `*.method` |
| **Prefix search** | `mimir search <root> --like "process"` | SQL LIKE prefix match |
| **Who calls this function?** | `mimir callers <root> <symbol>` | Default 2 levels deep. Use `--depth N` |
| **What does this function call?** | `mimir refs <root> --caller <name>` | Outbound references |
| **Most-called symbols (hotspots)** | `mimir refs <root> --hotspot` | Great for finding load-bearing code |
| **Dead code detection** | `mimir dead <root> --unexported` | `--unexported` reduces false positives |
| **Quick file inspection** | `mimir symbols <file>` | No index needed — parses on the fly |

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

## Important Caveats

1. **Always index first** — all query commands need an existing index (except `symbols` and `symbol` in file mode)
2. **Dead-code uses name-only matching** — false negatives possible for common names like `Open`, `Close`, `Error`. Use `--unexported` to reduce noise
3. **Framework entry points show as "dead"** — route handlers, decorators, fixtures are called by frameworks, not directly in code. These are expected false positives

## Full Command Reference

For complete flag documentation, output formats, and examples:
- **All commands**: Read `references/commands.md`
- **Workspace commands**: Read `references/workspaces.md`
