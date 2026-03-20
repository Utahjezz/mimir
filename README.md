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

- **9 CLI commands + workspace sub-commands** — index, search, symbol lookup, cross-reference tracing, dead-code detection, file tree, report; plus `workspace` to manage named collections of repos
- **6 languages** — Go, JavaScript, TypeScript, TSX, Python, C#
- **Incremental re-index** — mtime+size stat-skip; only changed files are re-parsed
- **Auto-refresh** — query commands transparently re-index stale files; no manual `mimir index` needed between edits
- **`--json` on every command** — pipe to `jq` or consume programmatically
- **Single binary** — requires Go 1.26+ and a C compiler (CGO, via tree-sitter)
- **FTS5 full-text search** — fuzzy symbol search with prefix wildcards (`proc*`)
- **Dot-notation** — `Class.method`, `*.method`, `Class.*` in `--name` / `--like`

---

## Installation

```bash
go install github.com/Utahjezz/mimir/cmd/mimir@latest
```

Or build from source:

```bash
git clone https://github.com/Utahjezz/mimir
cd mimir
go build -o mimir ./cmd/mimir
```

**Requirements**: Go 1.26+, and a C compiler (macOS: Xcode CLT — `xcode-select --install` · Linux: `gcc` · Windows: TDM-GCC or MSYS2).

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
--no-refresh    Skip automatic re-index before querying
```

### `mimir dead` flags

```
--type        <str>   Restrict to symbol type
--file        <str>   Filter by file path substring
--unexported         Only show unexported symbols (reduces false positives)
--json               Output as JSON
--no-refresh         Skip automatic re-index before querying
```

### `mimir refs` flags

```
--caller  <str>   Filter by caller symbol name
--callee  <str>   Filter by callee name
--file    <str>   Filter by caller file path
--json           Output as JSON
--no-refresh     Skip automatic re-index before querying
```

### Global flags (all commands)

```
--refresh-threshold <duration>   Minimum index age before a query triggers auto re-index
                                 (default 10s; e.g. 30s, 2m, 0s for always-refresh)
```

---

## Workspaces

A workspace is a named collection of repositories. Create one workspace per project or team, add multiple repos to it, and index them all with a single command.

```bash
# Create a workspace and set it as active
mimir workspace create myproject
mimir workspace use myproject

# Add repositories
mimir workspace add ~/code/backend
mimir workspace add ~/code/frontend

# Index all repos in the active workspace (2 concurrent by default)
mimir workspace index

# Show what's in the workspace
mimir workspace show
```

### Workspace commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `workspace create` | `mimir workspace create <name>` | Create a new workspace |
| `workspace use` | `mimir workspace use <name>` | Set the active workspace |
| `workspace add` | `mimir workspace add <path> [workspace]` | Add a repository to a workspace |
| `workspace remove` | `mimir workspace remove <path> [workspace]` | Remove a repository from a workspace |
| `workspace show` | `mimir workspace show [workspace]` | List repositories in a workspace |
| `workspace index` | `mimir workspace index [workspace] [flags]` | Index all repos in a workspace |

### `mimir workspace index` flags

```
--rebuild        Drop and rebuild each repo's index from scratch
--concurrency N  Number of repos to index in parallel (default 2)
--json           Output results as JSON (one object per repo)
```

### `mimir workspace show --json`

```bash
mimir workspace show --json | jq '.[].ID'
```

**DB location**: each workspace is stored at `~/.config/mimir/workspaces/<name>.db`
The active workspace name is stored in `~/.config/mimir/config.json`.

---

## Auto-refresh

Query commands (`search`, `report`, `refs`, `tree`, `callers`, `dead`, `symbol`) automatically re-index the repository when the index is older than the refresh threshold (default **10 seconds**). This means you rarely need to run `mimir index` manually between edits.

```bash
# Edit a file, then query immediately — auto-refresh picks up the change
vim pkg/indexer/walk.go
mimir search . --name "Run"       # re-indexes if index is > 10s old, then searches
```

**How it works**: before executing a query, mimir checks a single SQLite timestamp (`last_indexed_at` in the meta table). If the index is younger than the threshold, it proceeds directly. If stale, it runs the same incremental walk as `mimir index` (only changed files are re-parsed), then queries.

**Opt out** when you want raw speed or are running many queries in a tight loop:

```bash
mimir search . --name "Foo" --no-refresh
mimir dead   . --unexported --no-refresh
```

**Tune the threshold** globally for the entire command:

```bash
mimir --refresh-threshold=0s  search . --name "Foo"   # always re-index
mimir --refresh-threshold=5m  search . --name "Foo"   # re-index at most once per 5 min
```

> `mimir index` itself is still useful for the very first index build or when you want an explicit, unconditional walk (e.g. in CI).

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
5. **Query** — cobra commands open the index, run queries, and return results (read-only when the index is up to date)
6. **Auto-refresh** — query commands check `last_indexed_at` in meta; if stale they transparently re-run steps 1–4 and update SQLite before returning results

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
cmd/mimir/              ← entry point
internal/commands/      ← one file per subcommand
  workspace/            ← workspace subcommands (create, use, add, show, remove, index)
pkg/indexer/            ← core: walker, parser, store, queries, facade
  languages/            ← per-language tree-sitter grammars + queries
pkg/workspace/          ← workspace library: store, config, repository, index
```

---

## License

MIT
