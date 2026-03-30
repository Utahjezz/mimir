import { tool } from "@opencode-ai/plugin"

function mimirBin(): string {
  return `mimir`
}

// ---------------------------------------------------------------------------
// Shared helper – captures both stdout and stderr; on non-zero exit returns
// a human-readable error block instead of throwing a raw ShellError.
// ---------------------------------------------------------------------------
async function run(cmd: ReturnType<typeof Bun.$>): Promise<string> {
  const proc = await cmd.quiet().nothrow()
  const out  = proc.stdout.toString().trim()
  const err  = proc.stderr.toString().trim()
  if (proc.exitCode !== 0) {
    const detail = err || out || `(no output)`
    return `[mimir error] exit code ${proc.exitCode}\n${detail}`
  }
  // Some mimir commands write informational text to stderr on success (e.g. index progress).
  // Return stdout first; fall back to stderr if stdout is empty.
  return out || err
}

// ---------------------------------------------------------------------------
// mimir – index
// ---------------------------------------------------------------------------
export const index = tool({
  description:
    "Walk a directory, parse all supported source files (Go, TS, JS, Python, C#), " +
    "and persist symbols + call references to the mimir SQLite index. " +
    "Must be run before any query command.",
  args: {
    path: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root to index"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return index stats as JSON instead of plain text"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const flags = args.json ? ["--json"] : []
    return run(Bun.$`${bin} index ${args.path} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – symbols  (no index required)
// ---------------------------------------------------------------------------
export const symbols = tool({
  description:
    "Parse a single source file on-the-fly and list every extracted symbol " +
    "(name, type, line range). Does not require the index to exist.",
  args: {
    file: tool.schema
      .string()
      .describe("Path to the source file to parse"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    return run(Bun.$`${bin} symbols ${args.file}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – symbol
// ---------------------------------------------------------------------------
export const symbol = tool({
  description:
    "Parse a single source file on-the-fly, find the symbol by name, " +
    "and print its metadata (type, line range) and full source body. " +
    "When <root_or_file> is a directory, the index for that root is queried " +
    "to resolve the file path automatically — no need to know which file the " +
    "symbol lives in. Multiple matches are all printed; use --type to narrow. " +
    "When <root_or_file> is a file path, the file is parsed directly (no index needed). " +
    "Auto-refreshes the index (directory mode only) if it is older than --refresh-threshold " +
    "(default 10s); pass no_refresh=true to skip.",
  args: {
    root_or_file: tool.schema
      .string()
      .describe(
        "Repository root directory (index-aware, preferred) or path to a specific source file (legacy)"
      ),
    name: tool.schema
      .string()
      .describe("Exact symbol name to retrieve"),
    type: tool.schema
      .enum([
        "function",
        "method",
        "class",
        "interface",
        "type_alias",
        "enum",
        "namespace",
        "variable",
      ])
      .optional()
      .describe("Narrow results to a specific symbol type (index-aware mode only)"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON array (index-aware mode only)"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying (directory mode only)"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.type)       flags.push("--type", args.type)
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} symbol ${args.root_or_file} ${args.name} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – search
// ---------------------------------------------------------------------------
export const search = tool({
  description:
    "Query the symbol index for a repository root. " +
    "Supports exact name, prefix (--like), fuzzy FTS5 (--fuzzy), " +
    "type filter, and file-path filter. " +
    "Dot-notation: 'Class.method', '*.method', 'Class.*'. " +
    "Requires the index to exist (run mimir_index first). " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    name: tool.schema
      .string()
      .optional()
      .describe(
        "Exact symbol name match; supports dot-notation e.g. 'Class.method' or '*.method'"
      ),
    like: tool.schema
      .string()
      .optional()
      .describe("Symbol name prefix (SQL LIKE – no need to append %)"),
    fuzzy: tool.schema
      .string()
      .optional()
      .describe(
        "FTS5 full-text search with automatic camelCase/snake_case token splitting. " +
        "Plain words (e.g. 'process job') are split into identifier tokens and matched " +
        "against both the symbol name tokens and the body snippet (semantic tokens from the full AST subtree). " +
        "String literals in the body snippet are normalised: slash/hyphen/colon separators are treated as " +
        "word boundaries, so 'application/json' is searchable as 'application json'. " +
        "Use FTS5 operators (*  \"  :  ^) to bypass splitting and pass the query through unchanged."
      ),
    type: tool.schema
      .enum([
        "function",
        "method",
        "class",
        "interface",
        "type_alias",
        "enum",
        "namespace",
        "variable",
      ])
      .optional()
      .describe("Filter by symbol type"),
    file: tool.schema
      .string()
      .optional()
      .describe("Filter by file-path substring"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying (useful when running many queries in a tight loop)"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s' for always-refresh)"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.name)       flags.push("--name",  args.name)
    if (args.like)       flags.push("--like",  args.like)
    if (args.fuzzy)      flags.push("--fuzzy", args.fuzzy)
    if (args.type)       flags.push("--type",  args.type)
    if (args.file)       flags.push("--file",  args.file)
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} search ${args.root} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – report
// ---------------------------------------------------------------------------
export const report = tool({
  description:
    "Print a summary of the mimir index for a repository: file count, " +
    "symbol count, language breakdown, and symbol-type breakdown. " +
    "Requires the index to exist. " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return report as JSON"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} report ${args.root} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – refs
// ---------------------------------------------------------------------------
export const refs = tool({
  description:
    "Query the call-reference table of the mimir index. " +
    "Can filter by caller name, callee name, or file path. " +
    "Use --hotspot to print the top-N most-called symbols ranked by inbound call count. " +
    "Requires the index to exist. " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    caller: tool.schema
      .string()
      .optional()
      .describe("Filter by caller symbol name"),
    callee: tool.schema
      .string()
      .optional()
      .describe("Filter by callee symbol name"),
    file: tool.schema
      .string()
      .optional()
      .describe("Filter by caller file-path substring"),
    hotspot: tool.schema
      .boolean()
      .optional()
      .describe("Print the top-N most-called symbols ranked by inbound call count"),
    limit: tool.schema
      .number()
      .optional()
      .describe("Number of results to return for --hotspot (default 20)"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON (nested tree when depth > 1)"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.caller)  flags.push("--caller", args.caller)
    if (args.callee)  flags.push("--callee", args.callee)
    if (args.file)    flags.push("--file",   args.file)
    if (args.hotspot) flags.push("--hotspot")
    if (args.limit !== undefined) flags.push("--limit", String(args.limit))
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} refs ${args.root} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – imports
// ---------------------------------------------------------------------------
export const imports = tool({
  description:
    "Query the imports table of the mimir index. " +
    "Use --file to list all import statements in a specific source file, or " +
    "--module to find every file that imports a particular module/package path. " +
    "With no flags, all indexed import statements are returned. " +
    "Agent use-cases: dependency resolution (which package does symbol X come " +
    "from?), module boundary analysis (which files depend on an internal package?), " +
    "refactoring impact (what breaks if 'pkg/old' is renamed or removed?). " +
    "Requires the index to exist. " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    file: tool.schema
      .string()
      .optional()
      .describe("Filter by source file path — list all imports in this file"),
    module: tool.schema
      .string()
      .optional()
      .describe("Filter by imported module/package path — find all files that import this module"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON array of ImportRow objects"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
    workspace: tool.schema
      .string()
      .optional()
      .describe("Fan out query across all repos in this workspace (root is ignored when set)"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.file)       flags.push("--file",      args.file)
    if (args.module)     flags.push("--module",    args.module)
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    if (args.workspace)  flags.push("--workspace", args.workspace)
    return run(Bun.$`${bin} ${globalFlags} imports ${args.root} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – tree
// ---------------------------------------------------------------------------
export const tree = tool({
  description:
    "Print a directory tree derived from the mimir index, " +
    "with file and symbol counts per directory and language breakdown. " +
    "Requires the index to exist. " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    files: tool.schema
      .boolean()
      .optional()
      .describe("Show individual files under each directory"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return tree as JSON"),
    depth: tool.schema
      .number()
      .optional()
      .describe("Limit directory depth (0 = unlimited, useful for large repos)"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.files)      flags.push("--files")
    if (args.json)       flags.push("--json")
    if (args.depth !== undefined) flags.push("--depth", String(args.depth))
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} tree ${args.root} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – callers
// ---------------------------------------------------------------------------
export const callers = tool({
  description:
    "List every recorded call site in the mimir index that invokes a given symbol. " +
    "Callers at file scope (outside any function) are shown as '<file scope>'. " +
    "Use --depth N to recursively trace callers-of-callers up to N levels deep; " +
    "--depth 0 means unlimited depth (use with care on widely-called symbols). " +
    "Cycles are detected automatically and marked with [cycle]. " +
    "Requires the index to exist. " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    symbol: tool.schema
      .string()
      .describe("Symbol name whose callers to find"),
    depth: tool.schema
      .number()
      .int()
      .optional()
      .describe(
        "Recursively trace callers N levels deep. 0 = unlimited (prints a stderr warning). Default: 2 (one level of context over the flat list)"
      ),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON (nested tree when depth > 1)"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.depth !== undefined) flags.push("--depth", String(args.depth))
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} callers ${args.root} ${args.symbol} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace create
// ---------------------------------------------------------------------------
export const workspace_create = tool({
  description:
    "Create a new mimir workspace with the given name. " +
    "A workspace is a named collection of repositories stored at " +
    "~/.config/mimir/workspaces/<name>.db. " +
    "If the workspace already exists this is a no-op.",
  args: {
    name: tool.schema
      .string()
      .describe("Name for the new workspace"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    return run(Bun.$`${bin} workspace create ${args.name}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace use
// ---------------------------------------------------------------------------
export const workspace_use = tool({
  description:
    "Set the active workspace. Subsequent workspace commands that omit an " +
    "explicit workspace name will use this one by default. " +
    "The active name is persisted in ~/.config/mimir/config.json.",
  args: {
    name: tool.schema
      .string()
      .describe("Name of the workspace to activate"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    return run(Bun.$`${bin} workspace use ${args.name}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace add
// ---------------------------------------------------------------------------
export const workspace_add = tool({
  description:
    "Add a repository path to a workspace. " +
    "The path is resolved to an absolute path before storing. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    path: tool.schema
      .string()
      .describe("Absolute or relative path to the repository to add"),
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name to add the repo to (default: active workspace)"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const extra = args.workspace ? [args.workspace] : []
    return run(Bun.$`${bin} workspace add ${args.path} ${extra}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace remove
// ---------------------------------------------------------------------------
export const workspace_remove = tool({
  description:
    "Remove a repository from a workspace. " +
    "Returns an error if the path is not found in the workspace. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    path: tool.schema
      .string()
      .describe("Path of the repository to remove (must match the stored absolute path)"),
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name to remove the repo from (default: active workspace)"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const extra = args.workspace ? [args.workspace] : []
    return run(Bun.$`${bin} workspace remove ${args.path} ${extra}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace show
// ---------------------------------------------------------------------------
export const workspace_show = tool({
  description:
    "List all repositories in a workspace, including their ID, absolute path, " +
    "added-at timestamp, and last-indexed timestamp. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name to inspect (default: active workspace)"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON array"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const flags: string[] = []
    if (args.json) flags.push("--json")
    const extra = args.workspace ? [args.workspace] : []
    return run(Bun.$`${bin} workspace show ${extra} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace index
// ---------------------------------------------------------------------------
export const workspace_index = tool({
  description:
    "Index all repositories in a workspace. Repos are indexed incrementally " +
    "(mtime+size stat-skip) and concurrently up to --concurrency (default 2). " +
    "Continues on per-repo failure — one bad repo does not abort the run. " +
    "Use --rebuild to drop and recreate each index from scratch. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name to index (default: active workspace)"),
    rebuild: tool.schema
      .boolean()
      .optional()
      .describe("Drop and recreate each repo's index from scratch before indexing"),
    concurrency: tool.schema
      .number()
      .int()
      .optional()
      .describe("Number of repositories to index in parallel (default 2)"),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Output one JSON object per repo with name, path, duration, and error"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const flags: string[] = []
    if (args.rebuild)                    flags.push("--rebuild")
    if (args.concurrency !== undefined)  flags.push("--concurrency", String(args.concurrency))
    if (args.json)                       flags.push("--json")
    const extra = args.workspace ? [args.workspace] : []
    return run(Bun.$`${bin} workspace index ${extra} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace link
// ---------------------------------------------------------------------------
export const workspace_link = tool({
  description:
    "Declare a cross-repo symbol link from a symbol in one repository to a symbol " +
    "in another. Both symbols are validated against their repo indexes. " +
    "If a symbol name is ambiguous (multiple file matches), the command lists all " +
    "candidates and returns an error — re-run with src_file or dst_file to disambiguate. " +
    "Both repos must be registered in the workspace (mimir.workspace_add) and indexed " +
    "before calling this command. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    src_repo: tool.schema
      .string()
      .describe("Absolute or relative path to the source repository"),
    src_symbol: tool.schema
      .string()
      .describe("Exact name of the symbol in the source repository"),
    dst_repo: tool.schema
      .string()
      .describe("Absolute or relative path to the destination repository"),
    dst_symbol: tool.schema
      .string()
      .describe("Exact name of the symbol in the destination repository"),
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name (default: active workspace)"),
    src_file: tool.schema
      .string()
      .optional()
      .describe("File path suffix to disambiguate src_symbol when multiple matches exist"),
    dst_file: tool.schema
      .string()
      .optional()
      .describe("File path suffix to disambiguate dst_symbol when multiple matches exist"),
    note: tool.schema
      .string()
      .optional()
      .describe("Free-text note stored with the link (e.g. 'called synchronously on checkout')"),
    meta: tool.schema
      .array(tool.schema.string())
      .optional()
      .describe(
        "Structured key=value metadata pairs, e.g. ['protocol=grpc', 'transport=kafka']. " +
        "Multiple entries are allowed. Values are upserted — re-declaring a key overwrites it."
      ),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const flags: string[] = []
    if (args.src_file)  flags.push("--src-file", args.src_file)
    if (args.dst_file)  flags.push("--dst-file", args.dst_file)
    if (args.note)      flags.push("--note", args.note)
    for (const m of args.meta ?? []) flags.push("--meta", m)
    const extra = args.workspace ? [args.workspace] : []
    return run(
      Bun.$`${bin} workspace link ${args.src_repo} ${args.src_symbol} ${args.dst_repo} ${args.dst_symbol} ${extra} ${flags}`
    )
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace links
// ---------------------------------------------------------------------------
export const workspace_links = tool({
  description:
    "List cross-repo symbol links in a workspace. " +
    "By default filters to links whose source repo matches the current directory; " +
    "if cwd is not registered in the workspace all links are listed. " +
    "Pass from='' (empty string) to explicitly list all links regardless of cwd. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name (default: active workspace)"),
    from: tool.schema
      .string()
      .optional()
      .describe(
        "Filter by source repo path. Defaults to cwd. Pass empty string to list all links."
      ),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON array"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const flags: string[] = []
    if (args.from !== undefined) flags.push("--from", args.from)
    if (args.json)               flags.push("--json")
    const extra = args.workspace ? [args.workspace] : []
    return run(Bun.$`${bin} workspace links ${extra} ${flags}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – workspace unlink
// ---------------------------------------------------------------------------
export const workspace_unlink = tool({
  description:
    "Remove a cross-repo symbol link from a workspace by its numeric ID. " +
    "The ID is shown in mimir.workspace_links output (field 'ID' in JSON, '#N' in text). " +
    "Link metadata (key/value pairs) is cascade-deleted with the link. " +
    "Returns an error if the ID does not exist. " +
    "If workspace is omitted the current active workspace is used.",
  args: {
    id: tool.schema
      .number()
      .int()
      .describe("Numeric ID of the link to remove"),
    workspace: tool.schema
      .string()
      .optional()
      .describe("Workspace name (default: active workspace)"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const extra = args.workspace ? [args.workspace] : []
    return run(Bun.$`${bin} workspace unlink ${String(args.id)} ${extra}`)
  },
})

// ---------------------------------------------------------------------------
// mimir – dead
// ---------------------------------------------------------------------------
export const dead = tool({
  description:
    "Find symbols in the mimir index that are never called anywhere " +
    "(no entry in the refs table). Use --unexported to reduce false positives " +
    "from public-API symbols. Requires the index to exist. " +
    "Auto-refreshes the index if it is older than --refresh-threshold (default 10s); " +
    "pass no_refresh=true to skip the walk entirely.",
  args: {
    root: tool.schema
      .string()
      .describe("Absolute or relative path to the repository root"),
    type: tool.schema
      .enum([
        "function",
        "method",
        "class",
        "interface",
        "type_alias",
        "enum",
        "namespace",
        "variable",
      ])
      .optional()
      .describe("Restrict dead-code detection to a specific symbol type"),
    file: tool.schema
      .string()
      .optional()
      .describe("Filter by file-path substring"),
    unexported: tool.schema
      .boolean()
      .optional()
      .describe(
        "Only show unexported/private symbols (reduces false positives from public APIs)"
      ),
    json: tool.schema
      .boolean()
      .optional()
      .describe("Return results as JSON"),
    no_refresh: tool.schema
      .boolean()
      .optional()
      .describe("Skip the automatic re-index check before querying"),
    refresh_threshold: tool.schema
      .string()
      .optional()
      .describe("Override the minimum index age that triggers auto-refresh (Go duration string, e.g. '10s', '2m', '0s')"),
  },
  async execute(args, _context) {
    const bin = mimirBin()
    const globalFlags: string[] = []
    if (args.refresh_threshold) globalFlags.push("--refresh-threshold", args.refresh_threshold)
    const flags: string[] = []
    if (args.type)       flags.push("--type",  args.type)
    if (args.file)       flags.push("--file",  args.file)
    if (args.unexported) flags.push("--unexported")
    if (args.json)       flags.push("--json")
    if (args.no_refresh) flags.push("--no-refresh")
    return run(Bun.$`${bin} ${globalFlags} dead ${args.root} ${flags}`)
  },
})
