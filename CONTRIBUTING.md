# Contributing to Mimir

Thank you for your interest in contributing! This document covers everything you need to get started.

---

## Prerequisites

- **Go 1.26+** — [install](https://go.dev/dl/)
- **A C compiler** — required by tree-sitter bindings (CGO)
  - macOS: Xcode Command Line Tools (`xcode-select --install`)
  - Linux: `gcc` (`sudo apt install build-essential` or equivalent)
  - Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or MSYS2

---

## Development Setup

```bash
# Clone
git clone https://github.com/Utahjezz/mimir
cd mimir

# Build
go build -o mimir ./cmd/mimir

# Run tests
go test ./...
# Expected: 194 pass, 1 skip, 0 fail

# Try it on itself
./mimir index .
./mimir report .
./mimir dead . --unexported
```

---

## Project Structure

```
cmd/mimir/          ← entry point (main.go)
internal/commands/  ← one file per CLI subcommand
pkg/indexer/        ← core: walker, parser, store, queries, facade
  languages/        ← per-language tree-sitter grammars + queries
```

**Key files:**
- `pkg/indexer/facade.go` — public API (`MuncherFacade`)
- `pkg/indexer/registry.go` — language registration
- `pkg/indexer/store.go` — SQLite schema and CRUD
- `internal/commands/*.go` — Cobra subcommand implementations

---

## Code Standards

### General

- Follow standard Go conventions (`gofmt`, `go vet`)
- Wrap errors with context: `fmt.Errorf("opening index: %w", err)`
- Keep functions small and focused
- Table-driven tests in `*_test.go` files alongside source

### CLI Commands

Every command follows the same pattern:

```go
var myCmd = &cobra.Command{
    Use:   "command <path>",
    Short: "Brief description",
    RunE:  runCommand,
}

func runCommand(cmd *cobra.Command, args []string) error {
    db, err := indexer.OpenIndex(args[0])
    if err != nil {
        return fmt.Errorf("cannot open index: %w", err)
    }
    defer db.Close()

    // ...

    if jsonOutput {
        return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "...\n")
    return nil
}
```

- All commands must support a `--json` flag for scripting
- Use `cmd.OutOrStdout()` (not `os.Stdout`) so output is testable

### Naming

| Kind | Convention | Example |
|------|-----------|---------|
| Files | `snake_case` | `facade.go`, `indexer_test.go` |
| Packages | lowercase, short | `indexer`, `commands` |
| Exported types | `PascalCase` | `MuncherFacade`, `SymbolInfo` |
| Exported functions | `PascalCase` | `GetSymbols`, `OpenIndex` |
| Unexported | `camelCase` | `buildLangMap`, `runQuery` |
| DB tables | `snake_case` | `symbols`, `file_meta` |

### Security

- Use parameterized SQLite queries — no string interpolation in SQL
- Validate file paths before parsing (no directory traversal)
- Skip dot-dirs (`.git`, `.env`) and `node_modules`/`vendor`

---

## Adding a New Language

Two files and one registry entry:

**1. Create the language package:**

```
pkg/indexer/languages/<lang>/
├── language.go   ← Language() + Extensions()
└── queries.go    ← SymbolQuery, CallQuery, RefQuery constants
```

**2. Register it in `pkg/indexer/registry.go`:**

```go
var registeredLanguages = []languageDefinition{
    // existing entries...
    {
        language:   mylang.Language,
        query:      mylang.Queries,
        extensions: mylang.Extensions,
    },
}
```

**3. Add tests** covering symbol extraction for the new language.

---

## Pull Request Process

1. **Fork** the repository and create a branch from `main`
2. **Write tests** for any new behaviour
3. **Run the full test suite** — `go test ./...` must pass with 0 failures
4. **Keep PRs focused** — one feature or fix per PR
5. **Write a clear description** explaining what changed and why
6. PRs require at least one approving review before merge

### Commit Style

```
<type>: <short summary>

# Types: feat, fix, refactor, test, docs, chore
```

Examples:
```
feat: add Ruby language support
fix: skip symlinked directories during walk
docs: update CONTRIBUTING with CGO prereqs
```

---

## Reporting Issues

Before opening an issue:
- Search existing issues to avoid duplicates
- Try reproducing with the latest version (`go install github.com/Utahjezz/mimir/cmd/mimir@latest`)

When reporting a bug, include:
- Mimir version (`mimir --version`)
- OS and Go version
- The command you ran and the full output
- The repository being indexed (or a minimal reproduction)

Use the **bug report** template for bugs and the **feature request** template for ideas.

---

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
