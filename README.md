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

- **10 CLI commands + workspace sub-commands** — index, search, symbol lookup, cross-reference tracing, import tracking, dead-code detection, file tree, report; plus `workspace` to manage named collections of repos and declare cross-repo symbol links
- **6 languages** — Go, JavaScript, TypeScript, TSX, Python, C#
- **Incremental re-index** — mtime+size stat-skip; only changed files are re-parsed
- **Auto-refresh** — query commands transparently re-index stale files; no manual `mimir index` needed between edits
- **`--json` on every command** — pipe to `jq` or consume programmatically
- **Single binary** — requires Go 1.26+ and a C compiler (CGO, via tree-sitter)
- **FTS5 full-text search** — fuzzy symbol search with BM25 relevance ranking; automatic camelCase/snake_case splitting (`processOrder` matches both `process` and `order`); string literals normalised so `application/json` is searchable as `application json`
- **Dot-notation** — `Class.method`, `*.method`, `Class.*` in `--name` / `--like`

---

## Installation

### Homebrew (macOS and Linux)

```bash
brew install --cask Utahjezz/tap/mimir
```

To update:

```bash
brew upgrade --cask mimir
```

### Go install

```bash
go install github.com/Utahjezz/mimir/cmd/mimir@latest
```

### Build from source

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
| `symbols` | `mimir symbols <file>` | List all symbols in a file, plus its imports (no index needed) |
| `symbol` | `mimir symbol <file> <name>` | Show a symbol's metadata and source body |
| `search` | `mimir search <root> [flags]` | Search indexed symbols with filters |
| `report` | `mimir report <root>` | Summary: files, symbols, language breakdown |
| `refs` | `mimir refs <root> [flags]` | Query call-reference table |
| `imports` | `mimir imports <root> [flags]` | Query import/using statements |
| `tree` | `mimir tree <root> [--files]` | Directory tree with file/symbol counts |
| `callers` | `mimir callers <root> <symbol>` | All call sites that invoke a symbol |
| `dead` | `mimir dead <root> [flags]` | Symbols with no recorded callers |

### `mimir search` flags

```
--name   <str>   Exact symbol name (supports dot-notation: Class.method)
--like   <str>   Prefix match (LIKE)
--fuzzy  <str>   FTS5 full-text match; results ordered by BM25 relevance (best first).
                 camelCase/snake_case queries are split automatically: "processOrder"
                 matches symbols containing both "process" and "order". String literals
                 in the body snippet are normalised (slashes/hyphens/colons treated as
                 word boundaries), so "application/json" is searchable as "application json".
                 Use FTS5 operators (* " : ^) to bypass splitting and pass query unchanged.
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

### `mimir imports` flags

```
--file       <str>   All imports found in this file (relative path as indexed)
--module     <str>   All files that import this module path
--workspace  <name>  Fan out across all repos in the named workspace
--json               Output as JSON array of {file_path, import_path, alias, line}
--no-refresh         Skip automatic re-index before querying
```

With no filter flags, returns all recorded imports across the repo. Supported languages: Go, TypeScript, TSX, JavaScript, Python, C#.

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
# Next, set it as current: mimir workspace use myproject
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
| `workspace link` | `mimir workspace link <src-repo-id> <src-symbol> <dst-repo-id> <dst-symbol> [workspace]` | Declare a cross-repo symbol link |
| `workspace links` | `mimir workspace links [--from <repo>] [--src-symbol <name>] [--dst-symbol <name>] [--check] [workspace]` | List cross-repo symbol links |
| `workspace unlink` | `mimir workspace unlink <id> [workspace]` | Remove a cross-repo symbol link by ID |

### `mimir workspace index` flags

```
--rebuild        Drop and rebuild each repo's index from scratch
--concurrency N  Number of repos to index in parallel (default 2)
--json           Output results as JSON (one object per repo)
```

### `mimir workspace link` flags

```
--src-file <path>   Disambiguate src-symbol when multiple files match
--dst-file <path>   Disambiguate dst-symbol when multiple files match
--note     <text>   Free-text note stored with the link
--meta     <k=v>    Key/value metadata (repeatable: --meta protocol=grpc --meta transport=kafka)
```

### `mimir workspace links` flags

```
--from       <repo-id>   Filter links by source repo ID (defaults to cwd repo; lists all if cwd not in workspace)
--src-symbol <name>      Filter links by source symbol name (exact match)
--dst-symbol <name>      Filter links by destination symbol name (exact match)
--check                  Validate that symbols and file paths still exist; reports broken links
--json                   Output as JSON
```

### `mimir workspace show --json`

```bash
mimir workspace show --json | jq '.[].ID'
```

**DB location**: each workspace is stored at `~/.config/mimir/workspaces/<name>.db`
The active workspace name is stored in `~/.config/mimir/config.json`.

---

## Cross-repo Symbol Links

A cross-repo link is a manually declared mapping from a symbol in one repository to a symbol in another. Both sides are validated against their repo indexes when the link is created.

**Common uses**: documenting gRPC/HTTP boundaries (client call → server handler), marking shared interfaces implemented across repos, or capturing any meaningful cross-codebase relationship.

```bash
# First, find your repo IDs
mimir workspace show

# Declare that OrderService.PlaceOrder in the backend calls PaymentClient.Charge in payments
mimir workspace link backend-a1b2c3d4 OrderService.PlaceOrder payments-def45678 PaymentClient.Charge

# Attach metadata and a note
mimir workspace link backend-a1b2c3d4 OrderService.PlaceOrder payments-def45678 PaymentClient.Charge \
  --meta protocol=grpc --meta transport=kafka \
  --note "async via Kafka topic orders.placed"

# If a symbol name is ambiguous, use --src-file or --dst-file to disambiguate
mimir workspace link backend-a1b2c3d4 Shared payments-def45678 Shared \
  --src-file pkg/orders/handler.go \
  --dst-file pkg/payments/client.go

# List all links in the active workspace (filters to cwd repo by default)
mimir workspace links

# List links from a specific repo (by repo ID)
mimir workspace links --from backend-a1b2c3d4

# List all links across the workspace
mimir workspace links --from ""

# Filter by source symbol name
mimir workspace links --src-symbol OrderService.PlaceOrder

# Filter by destination symbol name
mimir workspace links --dst-symbol PaymentClient.Charge

# Combine filters: links from a specific repo that target a specific symbol
mimir workspace links --from backend-a1b2c3d4 --dst-symbol PaymentClient.Charge

# JSON output for scripting
mimir workspace links --json | jq '.[].SrcSymbol'

# Validate all links (check if symbols still exist and file paths match)
mimir workspace links --check

# Remove a link by ID
mimir workspace unlink 3
```

**Link output format:**
```
#1    OrderService.PlaceOrder (abc123)
      → PaymentClient.Charge (def456)
      note: async via Kafka topic orders.placed
      protocol=grpc
      transport=kafka
```

**Link validation output (`--check`):**
```
#1    MyFunc (abc123)
      → OtherFunc (def456)
      [CHECK] src: OK (pkg/orders.go)
      [CHECK] dst: OK (pkg/payments.go)

#2    MissingSymbol (abc123)
      → OtherFunc (def456)
      [CHECK] src: symbol "MissingSymbol" not found in repo
      [CHECK] dst: OK (pkg/payments.go)

⚠ 1 broken link(s) found. Run `mimir workspace unlink <id>` to remove.
```

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
| JavaScript | `.js` `.jsx` `.mjs` `.cjs` |
| TypeScript | `.ts` `.mts` `.cts` |
| TSX | `.tsx` |
| Python | `.py` `.pyw` |
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
3. **Parse** — tree-sitter extracts symbols, call references, and import/using statements per file
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
  workspace/            ← workspace subcommands (create, use, add, show, remove, index, link, links, unlink)
pkg/indexer/            ← core: walker, parser, store, queries, facade
  languages/            ← per-language tree-sitter grammars + queries
pkg/workspace/          ← workspace library: store, config, repository, index
```

---

## Claude Code Skill

Mimir ships with a [Claude Code](https://claude.com/claude-code) skill so that Claude can use mimir to explore any codebase. After installing the binary, run:

```bash
bash skills/install.sh
```

This copies the skill to `~/.claude/skills/mimir/`, making `/mimir` available in every Claude Code session — no Vercel Marketplace required.

### What the skill provides

Once installed, Claude can use mimir commands directly during conversations. Type `/mimir` or describe what you want to explore:

| Prompt | What Claude does |
|--------|-----------------|
| "index this repo" | `mimir index` + `mimir report` + `mimir tree` |
| "find symbol X" | `mimir search --name` or `mimir symbol` |
| "who calls this function?" | `mimir callers` with depth traversal |
| "what does this file import?" | `mimir imports --file` |
| "who imports this module?" | `mimir imports --module` |
| "show dead code" | `mimir dead --unexported` |
| "trace the call graph" | `mimir refs --caller` + `mimir callers` |
| "explore this codebase" | Full orientation workflow (index → report → tree → hotspots) |

### Supported workflows

- **First-time orientation** — index, report, tree, and hotspot analysis
- **Symbol lookup** — exact name, prefix, fuzzy, or dot-notation search
- **Impact analysis** — trace callers and callees before refactoring
- **Dependency analysis** — find all imports in a file or all files importing a module
- **Dead code audit** — find unexported symbols with no recorded callers
- **Cross-repo exploration** — workspace commands for multi-repo projects

---

## License

MIT
