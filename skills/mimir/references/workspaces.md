# Mimir Workspaces — Cross-Repo Exploration

Workspaces let you group multiple repos and search/trace across them.

## Setup

```bash
mimir workspace create myproject
mimir workspace use myproject
mimir workspace add ~/code/backend
mimir workspace add ~/code/payments
mimir workspace index                   # indexes all repos concurrently
```

## Commands

### `mimir workspace create <name>`
Create a new workspace. If it already exists, opens and verifies the connection (always prints "Workspace created").

### `mimir workspace use <name>`
Set the active workspace for subsequent commands.

### `mimir workspace list [--json]`
List all workspaces. Active one marked with `*`.

### `mimir workspace add <path> [workspace]`
Register a repo path in the active workspace.

### `mimir workspace remove <path> [workspace]`
Remove a repo from the workspace.

### `mimir workspace show [workspace] [--json]`
List all repos in a workspace with IDs and timestamps.

### `mimir workspace index [workspace] [--rebuild] [--concurrency N] [--json]`
Index all repos in a workspace. Incremental and concurrent (default: 2 concurrent).

### `mimir workspace delete <name> [--confirm]`
Permanently delete a workspace. Requires `--confirm`. Irreversible.

## Cross-Repo Symbol Links

Declare explicit connections between symbols in different repos (e.g., gRPC caller/callee).

### `mimir workspace link <src-repo-id> <src-symbol> <dst-repo-id> <dst-symbol> [workspace]`

| Flag | Description |
|------|-------------|
| `--src-file <path>` | Disambiguate source symbol by file |
| `--dst-file <path>` | Disambiguate dest symbol by file |
| `--note <text>` | Free-text note stored with the link |
| `--meta <k=v>` | Key/value metadata (repeatable) |

```bash
# First, find repo IDs with: mimir workspace show
mimir workspace link backend-a1b2c3d4 OrderService.PlaceOrder \
                     payments-def45678 PaymentClient.Charge \
  --meta protocol=grpc \
  --note "synchronous call on checkout"
```

### `mimir workspace links [workspace] [--from <path>] [--src-symbol <name>] [--dst-symbol <name>] [--check] [--json]`
List links. By default filters to links where the current working directory's repo appears as **either source or destination** — giving a complete picture of all cross-repo relationships for that repo. To list all links, run from a directory that is not registered in the workspace, or pass `--from ''`.

| Flag | Description |
|------|-------------|
| `--check` | Validate that symbols and file paths still exist in their respective repositories. Reports broken links (missing symbols or moved files). |
| `--from <path>` | Filter links by repository path — matches links where this repo is either source or destination |
| `--src-symbol <name>` | Filter by source symbol name (exact match) |
| `--dst-symbol <name>` | Filter by destination symbol name (exact match) |
| `--json` | Output as JSON array |

> **Note:** "No links found" when run from a registered repo means that repo has no declared
> cross-repo relationships (neither as caller nor callee). It does **not** mean the workspace
> is empty — run from an unregistered directory or pass `--from ''` to see all links.

```bash
# List all links with validation
mimir workspace links --check

# Output when broken links found:
# #1   MyFunc (repo-id)
#      → OtherFunc (repo-id)
#      [CHECK] src: symbol "MyFunc" not found in repo
#      [CHECK] dst: OK (pkg.go)
#
# ⚠ 1 broken link(s) found. Run `mimir workspace unlink <id>` to remove.
```

### `mimir workspace unlink <id> [workspace]`
Remove a link by numeric ID (shown in `workspace links` output).

## Link Discovery Protocol

**Run this only if the user confirms** (see `SKILL.md` → Cross-Repo Link Obligation for when to ask).

### Step 1 — Find candidate cross-repo calls

For each repo, inspect its outbound refs and check whether the callee names exist as symbols in any other workspace repo:

```bash
# See what a repo calls (scan callee_name column)
mimir refs <repo-path> --json

# For each callee name that looks like it might live in another repo:
mimir search <other-repo-path> --name "<callee-name>"
mimir search <other-repo-path> --fuzzy "<callee-name>"   # when casing differs
```

A **candidate link** exists when:
- Repo A calls a name that resolves as a symbol in Repo B
- The relationship is plausible (same domain, matching naming convention)

### Step 2 — Check existing links to avoid duplicates

```bash
mimir workspace links   # review what's already declared
```

Skip any candidate already covered by an existing link.

### Step 3 — Declare confirmed links

For each candidate you are confident about:

```bash
mimir workspace show   # get repo IDs

mimir workspace link <src-repo-id> <src-symbol> \
                     <dst-repo-id> <dst-symbol> \
  --note "<one sentence: what this connection represents>" \
  --meta protocol=<grpc|graphql|http|event|shared-type|...>
```

**What makes a good note:**
- States the direction: "A calls B to do X"
- Names the mechanism: GraphQL query, gRPC method, event, shared type
- Is useful to a future agent reading it cold

### Step 4 — Verify

```bash
mimir workspace links --check   # confirm links appear correctly and are not broken
```

### Common Rationalizations — Do Not Accept

| Excuse | Reality |
|--------|---------|
| "The connection is obvious" | Undeclared links don't survive the session |
| "I only found one relationship" | One link is worth declaring |
| "I'm not 100% sure" | Declare with a qualifying note; imperfect links beat no links |
| "I'll do it at the end" | The end is now — run the protocol before closing |

## Cross-Repo Search and Refs

```bash
# Search across all repos in workspace
mimir search --workspace myproject --name processJob
mimir search --workspace myproject --fuzzy "order placement"

# Refs across all repos (--hotspot not supported with --workspace)
mimir refs --workspace myproject --callee PaymentClient.Charge
```

## DB Locations

- Index DB: `~/.config/mimir/indexes/<repo-id>/index.db`
- Workspace DB: `~/.config/mimir/workspaces/<name>.db`
- Active workspace config: `~/.config/mimir/config.json`

All paths respect `$XDG_CONFIG_HOME`.
