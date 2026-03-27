# C# Namespace Indexing

**Date:** 2026-03-27
**Status:** Approved

## Problem

Mimir does not index C# `namespace` declarations. This means:

1. Searching `--name "DataAccess.*"` looks for a **parent class** named `DataAccess`, not a namespace — so classes inside `namespace DataAccess { }` are not found.
2. Namespaces don't appear as symbols — `--type namespace` returns nothing for C#.
3. There's no way to query "all symbols within namespace X".

TypeScript already has namespace support via `internal_module` captures. C# needs equivalent treatment.

## Solution: Namespace as Parent via `assignParents`

Reuse the existing `assignParents` stack mechanism. Namespaces become both indexed symbols and parent containers for top-level classes/interfaces/enums.

### Changes

#### 1. C# Tree-sitter Queries (`pkg/indexer/languages/csharp/queries.go`)

Add four patterns to `Queries`:

```
(namespace_declaration name: (qualified_name) @name) @namespace
(namespace_declaration name: (identifier) @name) @namespace
(file_scoped_namespace_declaration name: (qualified_name) @name) @namespace
(file_scoped_namespace_declaration name: (identifier) @name) @namespace
```

- `qualified_name` captures dotted names like `Netrunner.Cards`
- `identifier` captures simple names like `VantagePoint`
- `file_scoped_namespace_declaration` covers C# 10+ `namespace Foo.Bar;` syntax

#### 2. `assignParents` in `parser.go`

**Add `Namespace` to `parentTypes`:**

```go
var parentTypes = map[SymbolType]bool{
    Class:     true,
    Interface: true,
    Enum:      true,
    Namespace: true,  // NEW
}
```

**FQN concatenation in the stack:**

When pushing a `Namespace` onto the stack, if the current top is also a `Namespace`, concatenate: `outer.inner`. The `frame` struct needs a `symType` field to distinguish namespace frames from class frames.

```go
type frame struct {
    name    string
    endLine int
    symType SymbolType
}
```

When pushing a namespace:
- If stack top is a namespace → pushed name = `top.name + "." + current.name`
- Otherwise → pushed name = `current.name`

When a namespace is pushed with a concatenated FQN, the **symbol's own Name stays as declared** (e.g., `VantagePoint`), but the **parent field** gets the outer namespace FQN. The stack frame uses the FQN so that children (classes) receive the full path as their parent.

**Examples:**

**A) Simple namespace:**

```csharp
namespace DataAccess {
    public class UserRepository {
        public void Save(User u) {}
    }
}
```

| Symbol | Type | Parent |
|--------|------|--------|
| `DataAccess` | namespace | `` |
| `UserRepository` | class | `DataAccess` |
| `Save` | method | `UserRepository` |

**B) Nested namespaces (FQN concatenation):**

```csharp
namespace Company.Platform {
    namespace Services {
        public class OrderService {
            public void PlaceOrder() {}
        }
        public interface IPaymentGateway {
            void Charge(decimal amount);
        }
    }
}
```

| Symbol | Type | Parent | Stack frame name |
|--------|------|--------|-----------------|
| `Company.Platform` | namespace | `` | `Company.Platform` |
| `Services` | namespace | `Company.Platform` | `Company.Platform.Services` |
| `OrderService` | class | `Company.Platform.Services` | `OrderService` |
| `PlaceOrder` | method | `OrderService` | — |
| `IPaymentGateway` | interface | `Company.Platform.Services` | — |
| `Charge` | method | `IPaymentGateway` | — |

The stack frame for `Services` stores the FQN `Company.Platform.Services`, so children receive the full path as parent. The symbol row for `Services` itself has Name=`Services` and Parent=`Company.Platform`.

**C) File-scoped namespace (C# 10+):**

```csharp
namespace Company.Platform.Services;

public class NotificationService {
    public void Send(string message) {}
}

public enum Priority { Low, Medium, High }
```

| Symbol | Type | Parent |
|--------|------|--------|
| `Company.Platform.Services` | namespace | `` |
| `NotificationService` | class | `Company.Platform.Services` |
| `Send` | method | `NotificationService` |
| `Priority` | enum | `Company.Platform.Services` |

**D) Nested classes (existing behavior preserved):**

```csharp
namespace Models {
    public class Order {
        public class LineItem {
            public decimal Price { get; set; }
        }
        public void Submit() {}
    }
}
```

| Symbol | Type | Parent |
|--------|------|--------|
| `Models` | namespace | `` |
| `Order` | class | `Models` |
| `LineItem` | class | `Order` |
| `Price` | variable | `LineItem` |
| `Submit` | method | `Order` |

**E) Multiple namespaces in one file:**

```csharp
namespace Contracts {
    public interface ILogger {
        void Log(string msg);
    }
}

namespace Infrastructure {
    public class ConsoleLogger {
        public void Log(string msg) {}
    }
}
```

| Symbol | Type | Parent |
|--------|------|--------|
| `Contracts` | namespace | `` |
| `ILogger` | interface | `Contracts` |
| `Log` | method | `ILogger` |
| `Infrastructure` | namespace | `` |
| `ConsoleLogger` | class | `Infrastructure` |
| `Log` | method | `ConsoleLogger` |

#### 3. `ParseDotNotation` in `lookup.go`

Currently splits on the **first** dot. This breaks FQN queries:
- `"Company.Platform.Services.*"` → parent=`Company`, name=`Platform.Services.*` (wrong)

Change to split on the **last** dot:
- `"Company.Platform.Services.*"` → parent=`Company.Platform.Services`, name=`*` (correct)
- `"OrderService.PlaceOrder"` → parent=`OrderService`, name=`PlaceOrder` (unchanged, single dot)

Use `strings.LastIndexByte(s, '.')` instead of `strings.IndexByte(s, '.')`.

#### 4. `childTypes` update in `parser.go`

Add `Class`, `Interface`, `Enum` as child types so they receive the namespace as parent when nested inside one. Currently only `Method` and `Variable` are child types.

However, classes are also parent types (they contain methods). This is fine — `assignParents` first assigns parent, then pushes onto stack if it's a parent type. A class can be both a child (of a namespace) and a parent (of methods).

```go
var childTypes = map[SymbolType]bool{
    Method:    true,
    Variable:  true,
    Class:     true,      // NEW — receives namespace as parent
    Interface: true,      // NEW
    Enum:      true,      // NEW
    Namespace: true,      // NEW — nested namespace receives outer as parent
}
```

### Files Modified

| File | Change |
|------|--------|
| `pkg/indexer/languages/csharp/queries.go` | Add namespace query patterns |
| `pkg/indexer/parser.go` | `Namespace` in `parentTypes` and `childTypes`, FQN concat in `assignParents`, `symType` in frame |
| `pkg/indexer/lookup.go` | `ParseDotNotation` splits on last dot |
| `pkg/indexer/csharp_test.go` | Tests for namespace symbol extraction |
| `pkg/indexer/lookup_test.go` | Tests for FQN dot notation |

### Search Behavior After Change

Using the examples above:

| Query | Result |
|-------|--------|
| `--name "Company.Platform.Services.*"` | `OrderService`, `IPaymentGateway`, `NotificationService`, `Priority` — all symbols with that namespace as parent |
| `--name "OrderService.PlaceOrder"` | Method `PlaceOrder` in class `OrderService` (unchanged single-dot behavior) |
| `--type namespace` | `Company.Platform`, `Services`, `Company.Platform.Services`, `DataAccess`, `Models`, `Contracts`, `Infrastructure` |
| `--name "Services" --type namespace` | The `Services` namespace symbol |
| `--like "Company"` | Matches `Company.Platform` namespace (prefix) |
| `--name "Models.*"` | `Order` class — top-level class in `Models` namespace |
| `--name "Order.*"` | `LineItem` class + `Submit` method — children of `Order` class |

### No Schema Migration Needed

No new columns or tables. The existing `parent` field and `type` column are sufficient. Only `indexVersion` bump is needed to force a re-index so existing C# files get namespace data.

### Caveats

1. **`childTypes` broadening** — adding Class/Interface/Enum to `childTypes` means they'll receive parent from ANY container, not just namespaces. In practice this is correct: a class nested inside another class should have the outer class as parent (C# supports nested classes).
2. **ParseDotNotation last-dot split** — this is a behavior change. `"A.B.C"` previously → parent=`A`, name=`B.C`. Now → parent=`A.B`, name=`C`. This is the correct semantic for FQN queries and doesn't break single-dot queries.
3. **File-scoped namespaces** — these have no end line (they scope the entire file). The tree-sitter node should encompass the whole file, so the stack frame's endLine will be the last line of the file, which is correct.
4. **`alias_qualified_name` not covered** — the C# grammar allows `global::SomeNamespace` as a namespace name via `alias_qualified_name`. The query patterns above don't capture this form. This is an uncommon edge case; if encountered, the namespace is silently skipped. Can be added later if needed.
5. **FQN concatenation is namespace-only** — the FQN concatenation (`outer.inner`) only applies when stacking namespace-on-namespace frames. Class frames are never concatenated: a class `Order` inside namespace `Models` has frame name `Order` (not `Models.Order`). Methods inside `Order` get parent=`Order`, not `Models.Order`.
