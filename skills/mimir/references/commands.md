# Mimir Command Reference

## Table of Contents
- [Global Flags](#global-flags)
- [mimir --version](#mimir---version)
- [mimir index](#mimir-index)
- [mimir symbols](#mimir-symbols)
- [mimir symbol](#mimir-symbol)
- [mimir search](#mimir-search)
- [mimir report](#mimir-report)
- [mimir refs](#mimir-refs)
- [mimir tree](#mimir-tree)
- [mimir callers](#mimir-callers)
- [mimir dead](#mimir-dead)

---

## Global Flags

| Flag | Description |
|------|-------------|
| `--refresh-threshold <dur>` | Minimum index age before auto re-index (e.g. `10s`, `2m`, `0s`). Default: `10s`. Pass `0s` to force re-walk. |

---

## `mimir --version`

```bash
mimir --version
mimir -v
```

Output: `mimir dev (index schema v5)`

---

## `mimir index`

Walk path, parse all supported files, persist symbols to SQLite.

```bash
mimir index <path> [--rebuild] [--json]
```

| Flag | Description |
|------|-------------|
| `--rebuild` | Drop existing index and reindex from scratch (required after schema upgrades) |
| `--json` | Output stats as JSON |

**Text:** `added 142  updated 3  unchanged 891  removed 0  errors 0`

**JSON:** `{"Added":142,"Updated":3,"Unchanged":891,"Removed":0,"Errors":0,"FileErrors":[]}`

---

## `mimir symbols`

Parse a file on-the-fly (no index needed) and list every symbol.

```bash
mimir symbols <file>
```

**Output:**
```
function     NewMuncher                               12-18
method       GetSymbols                               20-45
```

---

## `mimir symbol`

Find symbol by name and print metadata + full source body.

```bash
mimir symbol <root-or-file> <name> [--type <type>] [--no-refresh] [--json]
```

| Flag | Description |
|------|-------------|
| `--type <str>` | Narrow to specific symbol type when multiple matches exist |
| `--no-refresh` | Skip automatic re-index |
| `--json` | Output as JSON array (index-aware mode only; ignored in file mode) |

**Index-aware mode** (preferred): `<root-or-file>` is a directory — resolves file automatically and honors `--json`.
**File mode**: `<root-or-file>` is a file path — parses directly without index; `--json` is ignored.

**Output:**
```
name:  GetSymbols
type:  method
file:  pkg/indexer/facade.go
lines: 20-45

func (m *MuncherFacade) GetSymbols(path string, code []byte) ([]SymbolInfo, error) {
    ...
}
```

---

## `mimir search`

Query the symbol index. With no flags, returns all indexed symbols.

```bash
mimir search [root] [--name <exact>] [--like <prefix>] [--fuzzy <fts5>]
                    [--type <type>] [--file <path>] [--limit N]
                    [--workspace <name>] [--no-refresh] [--json]
```

| Flag | Description |
|------|-------------|
| `--name <str>` | Exact symbol name match |
| `--like <str>` | Symbol name prefix (SQL `LIKE` — trailing `%` is added automatically; do not include it) |
| `--fuzzy <str>` | FTS5 match: camelCase/snake_case splitting, multi-word, body snippet; results ordered by BM25 relevance |
| `--type <str>` | Filter by symbol type |
| `--file <str>` | Exact match on indexed relative file path (e.g. `pkg/indexer/facade.go`) |
| `--limit N` | Maximum number of results to return (`0` = unlimited, default) |
| `--workspace <name>` | Fan out search across all repos in workspace (`[root]` is ignored) |
| `--no-refresh` | Skip automatic re-index |
| `--json` | Output as JSON |

**Fuzzy behavior:** When query has no FTS5 operators, mimir auto-splits camelCase/snake_case tokens and applies prefix matching against name tokens AND body snippet. String literals in body snippets are normalised — slashes, hyphens, and colons are treated as word boundaries, so `application/json` is searchable as `application json`. Results are ordered by BM25 relevance (best match first). With FTS5 operators (`*`, `"`, `:`), query is passed through unchanged.

**Dot-notation:** `Class.method` (specific), `*.method` (any class), `Class.*` (all members).

**Text output:**
```
method       MuncherFacade.GetSymbols          pkg/indexer/facade.go        20-45
function     NewMuncher                        pkg/indexer/facade.go        12-18
```

---

## `mimir report`

Summary of the index: file count, symbol count, language and type breakdown.

```bash
mimir report <root> [--no-refresh] [--json]
```

---

## `mimir refs`

Query call-reference table. Use `--hotspot` for most-called symbols.

```bash
mimir refs [root] [--caller <name>] [--callee <name>] [--file <path>]
                  [--hotspot] [--limit N]
                  [--workspace <name>] [--no-refresh] [--json]
```

| Flag | Description |
|------|-------------|
| `--caller <str>` | Filter by caller symbol name |
| `--callee <str>` | Filter by callee name |
| `--file <str>` | Filter by caller file path |
| `--hotspot` | Top-N most-called symbols by inbound call count |
| `--limit N` | Number of results for `--hotspot` (default 20) |
| `--workspace <name>` | Fan out across workspace (mutually exclusive with `--hotspot`) |

**Hotspot output:**
```
rank   callee                                    call_count  file
1      Errorf                                           384  (external)
2      GetSymbols                                        69  pkg/indexer/facade.go
```

---

## `mimir tree`

Directory tree from the index with file and symbol counts.

```bash
mimir tree <root> [--files] [--depth N] [--no-refresh] [--json]
```

| Flag | Description |
|------|-------------|
| `--files` | Show individual files under each directory |
| `--depth N` | Limit to top N directory levels (0 = unlimited) |

---

## `mimir callers`

List every call site that invokes a symbol. Use `--depth` for recursive tracing.

```bash
mimir callers <root> <symbol> [--depth N] [--no-refresh] [--json]
```

| Flag | Description |
|------|-------------|
| `--depth N` | Recursively trace callers N levels deep. `0` = unlimited. Default: 2 |

**Output (depth 2):**
```
pkg/indexer/walk.go                      processJob           line 94
  cmd/mimir/main.go                      main                 line 12
pkg/indexer/walk_test.go                 <file scope>         line 41
```

Cycles detected automatically and shown with `[cycle]`.

---

## `mimir dead`

Find symbols never called anywhere in the index.

```bash
mimir dead <root> [--type <type>] [--file <path>] [--unexported] [--no-refresh] [--json]
```

| Flag | Description |
|------|-------------|
| `--type <str>` | Restrict to a symbol type |
| `--file <str>` | Filter by file path substring |
| `--unexported` | Only unexported symbols (reduces false positives from public APIs) |

**Warning:** Uses name-only JOIN matching. Symbols with common names (`Open`, `Close`, `Error`, `Read`, `Write`, `String`) may be incorrectly excluded.

## Symbol Types

| Type | Description |
|------|-------------|
| `function` | Top-level / free function |
| `method` | Method on a class or struct |
| `class` | Class declaration |
| `interface` | Interface declaration |
| `type_alias` | Type alias |
| `enum` | Enum declaration |
| `namespace` | Namespace / module declaration |
| `variable` | Top-level variable / constant |
