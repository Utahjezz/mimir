# Mimir

Language-agnostic code indexer. Parse a repository once with tree-sitter, store symbols and call references in SQLite, then explore the codebase via a fast CLI — no daemon, no LSP, no runtime dependencies.

```bash
mimir index ./myrepo
mimir search ./myrepo --name "processJob"
mimir callers ./myrepo processJob
mimir dead ./myrepo --unexported
```

---

## Features

- **9 CLI commands** — index, search, symbol lookup, cross-reference tracing, dead-code detection, file tree, report
- **6 languages** — Go, JavaScript, TypeScript, TSX, Python, C#
- **Incremental re-index** — mtime+size stat-skip; only changed files are re-parsed
- **`--json` on every command** — pipe to `jq` or consume programmatically
- **Single binary** — pure Go, no CGo, no C toolchain required
- **FTS5 full-text search** — fuzzy symbol search with prefix wildcards (`proc*`)
- **Dot-notation** — `Class.method`, `*.method`, `Class.*` in `--name` / `--like`

---

## Installation

```bash
go install github.com/utahjezz/mimir/cmd/mimir@latest
```

Or build from source:

```bash
git clone https://github.com/utahjezz/mimir
cd mimir
go build -o mimir ./cmd/mimir
```

**Requirements**: Go 1.21+. No other dependencies.

---

## Quick Start

```bash
# 1. Index a repository (run once; re-run to update)
mimir index ./myrepo

# 2. See what's in it
mimir report ./myrepo

# 3. Find a symbol
mimir search ./myrepo --name "NewMuncher"

# 4. Show its source
mimir symbol pkg/indexer/facade.go NewMuncher

# 5. Who calls it?
mimir callers ./myrepo NewMuncher

# 6. What's never called?
mimir dead ./myrepo --unexported
```

---

## Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `index` | `mimir index <path>` | Walk and index all supported source files |
| `symbols` | `mimir symbols <file>` | List all symbols in a file (no index needed) |
| `symbol` | `mimir symbol <file> <name>` | Show a symbol's metadata and source body |
| `search` | `mimir search <root> [flags]` | Search indexed symbols with filters |
| `report` | `mimir report <root>` | Summary: files, symbols, language breakdown |
| `refs` | `mimir refs <root> [flags]` | Query call-reference table |
| `tree` | `mimir tree <root> [--files]` | Directory tree with file/symbol counts |
| `callers` | `mimir callers <root> <symbol>` | All call sites that invoke a symbol |
| `dead` | `mimir dead <root> [flags]` | Symbols with no recorded callers |

### `mimir search` flags

```
--name   <str>   Exact symbol name (supports dot-notation: Class.method)
--like   <str>   Prefix match (LIKE)
--fuzzy  <str>   FTS5 match — use * for prefix: "proc*"
--type   <str>   Filter by type: function | method | class | interface |
                               type_alias | enum | namespace | variable
--file   <str>   Filter by file path substring
--json          Output as JSON
```

### `mimir dead` flags

```
--type        <str>   Restrict to symbol type
--file        <str>   Filter by file path substring
--unexported         Only show unexported symbols (reduces false positives)
--json               Output as JSON
```

### `mimir refs` flags

```
--caller  <str>   Filter by caller symbol name
--callee  <str>   Filter by callee name
--file    <str>   Filter by caller file path
--json           Output as JSON
```

---

## Supported Languages

| Language | Extensions |
|----------|-----------|
| Go | `.go` |
| JavaScript | `.js` `.mjs` `.cjs` |
| TypeScript | `.ts` `.mts` `.cts` |
| TSX | `.tsx` |
| Python | `.py` |
| C# | `.cs` |

Files with any other extension are silently skipped.

---

## Symbol Types

`function` · `method` · `class` · `interface` · `type_alias` · `enum` · `namespace` · `variable`

---

## JSON Output

Every command supports `--json` for scripting:

```bash
# Count dead unexported functions
mimir dead ./myrepo --unexported --type function --json | jq 'length'

# All method names in a package
mimir search ./myrepo --type method --file pkg/indexer/ --json | jq '.[].Name'

# Language breakdown
mimir report ./myrepo --json | jq '.Languages'
```

---

## How It Works

1. **Walk** — directory tree skipping dot-dirs (`.git`, `.env`, …) and `node_modules`/`vendor`
2. **Stat-skip** — compare mtime+size against stored `FileMeta`; skip if unchanged
3. **Parse** — tree-sitter extracts symbols and call references per file
4. **Write** — single collector goroutine writes to SQLite (no locking errors)
5. **Query** — cobra commands open the index read-only and return results

**DB location**: `~/.config/mimir/indexes/<repo-id>/index.db`
(override with `$XDG_CONFIG_HOME`)

---

## Adding a New Language

Two files and one registry entry:

```
pkg/indexer/languages/<lang>/
├── language.go    ← Language() + Extensions()
└── queries.go     ← SymbolQuery, CallQuery, RefQuery constants
```

Then add one entry to `buildLangMap()` in `pkg/indexer/registry.go`.

---

## Development

```bash
# Run all tests
go test ./...
# Expected: 194 pass, 1 skip (git-head test skipped — no commits yet), 0 fail

# Build binary
go build -o mimir ./cmd/mimir

# Index this repo and explore it
./mimir index .
./mimir report .
./mimir dead . --unexported
```

---

## Project Structure

```
cmd/mimir/          ← entry point
internal/commands/  ← one file per subcommand
pkg/indexer/        ← core: walker, parser, store, queries, facade
  languages/        ← per-language tree-sitter grammars + queries
.opencode/skills/mimir/SKILL.md  ← AI agent usage guide
```

---

## License

MIT
