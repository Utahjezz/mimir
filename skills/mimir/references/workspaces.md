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

### `mimir workspace links [workspace] [--from <path>] [--src-symbol <name>] [--dst-symbol <name>] [--json]`
List links. Defaults to filtering by the repo containing the current working directory. To list all links, run from a directory that is not registered in the workspace.

### `mimir workspace unlink <id> [workspace]`
Remove a link by numeric ID (shown in `workspace links` output).

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
