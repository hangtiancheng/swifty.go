---
name: lark-orm
description: >
  Knex-style chainable MongoDB ORM for Go (lark_orm module,
  github.com/hangtiancheng/lark-go/lark_orm). Use when writing or modifying Go
  code that calls lark_orm.NewEngine, engine.Collection, engine.Model, the
  chainable Query builder (Where, WhereIn, WhereNotIn, WhereNull, WhereNotNull,
  WhereBetween, OrWhere, OrderBy, Limit, Offset, Select, Insert, Find, First,
  Update, Delete, Count, Exists, EnsureIndexes, DropCollection), aggregations
  (Sum, Avg, Min, Max, Distinct, Pluck), MongoDB session transactions via
  engine.Transaction, auto-increment counters via engine.NextSequence,
  struct-to-collection name derivation (CollectionName), or any import of
  github.com/hangtiancheng/lark-go/lark_orm. Also use when the user asks about
  Knex-style query chaining in Go for MongoDB, BSON filter construction, the
  Engine lifecycle, or replacing knex+mysql usage with lark_orm+mongodb. Do not
  use for GORM, sqlx, ent, raw mongo-driver code that does not import lark_orm,
  or any non-MongoDB datastore.
---

# lark_orm

A Knex-inspired, chainable query builder ORM for MongoDB in Go. Provides a
fluent API for CRUD, aggregation, transactions, and auto-incrementing sequences
on top of the official `go.mongodb.org/mongo-driver`.

- Module path: `github.com/hangtiancheng/lark-go/lark_orm`
- Source root: `lark_orm/`
- Go toolchain: 1.26+
- All exported types live directly in the `lark_orm` package (flat layout, no
  sub-packages).

## Architecture overview

```
Engine (engine.go)
  |-- *mongo.Client            (lifecycle)
  |-- *mongo.Database          (lifecycle)
  |-- mongo.Session            (set only inside Transaction)
  |
  |-- Client() / Database() / DatabaseName()
  |-- Close(ctx) / DropDatabase(ctx)
  |-- Collection(name) -> *Query
  |-- Model(value)     -> *Query     // collection name derived via reflection
  |-- Transaction(ctx, fn)            // session-scoped sub-Engine
  |-- NextSequence(ctx, name)         // atomic int64 counter

Query (query.go, mutable, chainable)
  |-- conditions []condition
  |-- orGroups   [][]condition
  |-- sort, limit, skip, fields
  |
  |-- Where / WhereIn / WhereNotIn / WhereNull / WhereNotNull / WhereBetween / OrWhere
  |-- OrderBy / Limit / Offset / Select
  |-- Insert / First / Find / Update / Delete / Count / Exists
  |-- Sum / Avg / Min / Max / Distinct / Pluck
  |-- EnsureIndexes / DropCollection
```

`Query` is intentionally a small mutable struct. Each chain method appends to
its slices and returns the same receiver. See "Pitfalls" for the implications.

## Core types

### Engine

Wraps a MongoDB client plus a single database handle.

```go
func NewEngine(ctx context.Context, uri string, database string) (*Engine, error)
```

`NewEngine` connects, pings, and returns a ready engine. It returns errors for
empty URI, empty database name, connect failure, or ping failure. On ping
failure the client is disconnected before returning, so no handle leaks.

All Engine methods are nil-safe on the receiver: calling `Client()`,
`Database()`, `DatabaseName()`, `Close()`, or `DropDatabase()` on a nil
`*Engine` returns the zero value or nil error rather than panicking.

| Method       | Signature                                                                    | Notes                                                                                    |
| ------------ | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| NewEngine    | `(ctx, uri, database) (*Engine, error)`                                      | Constructor. Pings before returning.                                                     |
| Client       | `() *mongo.Client`                                                           | Underlying driver client.                                                                |
| Database     | `() *mongo.Database`                                                         | Active database handle.                                                                  |
| DatabaseName | `() string`                                                                  | Database name passed to `NewEngine`.                                                     |
| Collection   | `(name string) *Query`                                                       | Empty name yields a Query whose execution methods return `ErrCollectionRequired`.        |
| Model        | `(value interface{}) *Query`                                                 | Equivalent to `Collection(CollectionName(value))`.                                       |
| Close        | `(ctx context.Context) error`                                                | Disconnect the client.                                                                   |
| DropDatabase | `(ctx context.Context) error`                                                | Drop the entire database. Destructive.                                                   |
| NextSequence | `(ctx context.Context, name string) (int64, error)`                          | Atomic counter on the hard-coded `counters` collection. First call for a name returns 1. |
| Transaction  | `(ctx context.Context, fn func(sc context.Context, tx *Engine) error) error` | Session transaction. `fn` MUST use `sc` as ctx; see "Transactions" below.                |

### Query

Chainable query builder. Obtained from `engine.Collection(name)` or
`engine.Model(value)`. All condition methods return the receiver for chaining
and have no error return; errors surface only at execution time.

Condition methods:

| Method       | Signature                                | Behavior                                                                                 |
| ------------ | ---------------------------------------- | ---------------------------------------------------------------------------------------- |
| Where        | `(field, value)` or `(field, op, value)` | Two-arg form is equality. Three-arg form uses the operator string.                       |
| WhereIn      | `(field, values)`                        | `$in`. `values` should be a slice; passed straight to BSON.                              |
| WhereNotIn   | `(field, values)`                        | `$nin`.                                                                                  |
| WhereNull    | `(field)`                                | Sets `field: nil`.                                                                       |
| WhereNotNull | `(field)`                                | Sets `field: {$ne: nil}`.                                                                |
| WhereBetween | `(field, low, high)`                     | Sets `{$gte: low, $lte: high}` and merges with other ops on the same field.              |
| OrWhere      | `(field, value)` or `(field, op, value)` | Each call appends a separate `$or` branch.                                               |
| OrderBy      | `(field, direction?)`                    | Direction is `"asc"` (default) or `"desc"` / `"DESC"`. Other casings are treated as asc. |
| Limit        | `(n int64)`                              | Only applied when `n > 0`.                                                               |
| Offset       | `(n int64)`                              | Only applied when `n > 0`. Applies to both `Find` and `First`.                           |
| Select       | `(fields ...string)`                     | Inclusive projection only. Repeated calls replace, not merge.                            |

Comparison operators recognized in `Where(field, op, value)` (case-sensitive,
exact match against `opMap`):

```
=  !=  <>  >  >=  <  <=  $in  $nin
```

Any operator string not in the map is passed through to MongoDB verbatim, so
`Where("name", "$regex", "^A")` works. This also means typos like `>==` reach
the driver and cause a server-side error.

Execution methods:

| Method         | Signature                                          | Behavior                                                                                                           |
| -------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Insert         | `(ctx, docs ...interface{}) (InsertResult, error)` | One doc uses `InsertOne`; multiple docs use `InsertMany`. Requires at least one doc.                               |
| First          | `(ctx, out interface{}) error`                     | Backed by `FindOne`. Honors sort, skip, projection. Returns `mongo.ErrNoDocuments` when nothing matches.           |
| Find           | `(ctx, out interface{}) error`                     | Backed by `Find` + `cursor.All`. Loads the entire result set into `out`.                                           |
| Update         | `(ctx, update interface{}) (int64, error)`         | Backed by `UpdateMany`. Returns `ModifiedCount`. Auto-wraps `bson.M` without `$`-prefixed keys into `{$set: ...}`. |
| Delete         | `(ctx) (int64, error)`                             | Backed by `DeleteMany`. Returns `DeletedCount`.                                                                    |
| Count          | `(ctx) (int64, error)`                             | Backed by `CountDocuments`.                                                                                        |
| Exists         | `(ctx) (bool, error)`                              | Implemented as `Count(ctx) > 0`.                                                                                   |
| EnsureIndexes  | `(ctx, []mongo.IndexModel) ([]string, error)`      | No-op (returns `nil, nil`) when the slice is empty.                                                                |
| DropCollection | `(ctx) error`                                      | Drops the underlying collection.                                                                                   |

`Update`'s `$set` auto-wrap fires only when the argument is the concrete type
`bson.M` and none of its keys start with `$`. If you pass `bson.D`,
`map[string]interface{}`, or a struct, the value is forwarded as-is and the
driver will reject documents lacking an update operator.

### InsertResult

```go
type InsertResult struct {
    InsertedIDs   []interface{}
    InsertedCount int64
}
```

`InsertedCount` is derived from `len(InsertedIDs)` in both the single and
multi-document paths.

### Aggregation methods

All aggregation methods reuse the current condition chain. They build the
pipeline `[{$match: filter}, {$group: {_id: null, result: <accumulator>}}]` and
read the single grouped result.

| Method   | Signature                             | Behavior                                                                                      |
| -------- | ------------------------------------- | --------------------------------------------------------------------------------------------- |
| Sum      | `(ctx, field) (float64, error)`       | `$sum`. Returns `(0, nil)` when no documents match.                                           |
| Avg      | `(ctx, field) (float64, error)`       | `$avg`. Returns `(0, nil)` when no documents match.                                           |
| Min      | `(ctx, field) (float64, error)`       | `$min`. Numeric only; non-numeric fields will fail to decode into `float64`.                  |
| Max      | `(ctx, field) (float64, error)`       | `$max`. Same numeric-only caveat.                                                             |
| Distinct | `(ctx, field) ([]interface{}, error)` | Direct call to `Collection.Distinct`. Caller must type-assert each element.                   |
| Pluck    | `(ctx, field, out) error`             | Mutates `q.fields = []string{field}` then calls `Find`. Do not reuse the Query after `Pluck`. |

### Collection naming

`CollectionName(value)` derives a collection name from a struct type:

1. Unwrap pointers, then unwrap slice/array element, then unwrap pointers again
2. If the resulting kind is not a struct, return `""`
3. Convert the type's `Name()` from `CamelCase` to `snake_case` by inserting an
   underscore before each non-leading uppercase letter
4. Pluralize with these rules in order:
   - consonant + `y` -> drop `y`, add `ies`
   - ends with `s`, `x`, `sh`, `ch` -> add `es`
   - otherwise -> add `s`

Worked examples:

```
User           -> users
Address        -> addresses
Category       -> categories
Day            -> days
ChatHistory    -> chat_histories
[]testCity     -> test_cities
&testUser{}    -> test_users
42 (non-struct)-> ""
```

Limitations to be aware of:

- Acronyms are split letter-by-letter: `HTTPServer` becomes `h_t_t_p_server`,
  not `http_servers`. For acronym-heavy names, call `engine.Collection("explicit_name")`.
- Irregular plurals are not handled (`Person -> persons`, `Hero -> heros`,
  `Child -> childs`). Override via `engine.Collection(...)`.
- There is no `CollectionNamer` interface escape hatch; the only override is to
  use `Collection` instead of `Model`.

### Filter construction (`filter.go`)

`(*Query).buildFilter()` translates the condition chain into a `bson.M`:

- `op == "="` sets `field: value` directly on the filter map
- `op == "null"` sets `field: nil`
- `op == "notNull"` merges `{$ne: nil}` onto the field
- `op == "between"` merges `{$gte: low, $lte: high}` onto the field
- Any other op is looked up in `opMap` (or passed through if absent), then
  merged onto the field via `mergeFieldOp`
- Multiple non-equality ops on the same field merge into one sub-document
  (`Where("age", ">", 18).Where("age", "<", 30)` produces
  `{age: {$gt: 18, $lt: 30}}`)
- `OrWhere` produces `{$or: [<base>, <or-group-1>, <or-group-2>, ...]}` where
  `<base>` is the filter built from the main `Where*` chain

`buildProjection()` returns `nil` when no `Select` was called, otherwise a
`bson.M` mapping each field to `1` (inclusive projection only).

## Transactions

```go
err := engine.Transaction(ctx, func(sc context.Context, tx *lark_orm.Engine) error {
    if _, err := tx.Collection("accounts").
        Where("_id", fromID).
        Update(sc, bson.M{"$inc": bson.M{"balance": -amount}}); err != nil {
        return err
    }
    if _, err := tx.Collection("accounts").
        Where("_id", toID).
        Update(sc, bson.M{"$inc": bson.M{"balance": amount}}); err != nil {
        return err
    }
    return nil
})
```

Contract:

- `tx` shares the same `*mongo.Client` and `*mongo.Database` as the outer
  engine; it carries the session privately
- The callback MUST pass `sc` (the `mongo.SessionContext` re-typed as
  `context.Context`) into every lark_orm execution method that should
  participate in the transaction. Using `ctx` from the outer scope silently
  skips the session
- Returning `nil` from the callback commits; returning an error aborts and
  propagates the error to the caller of `Transaction`
- Requires MongoDB deployed as a replica set or sharded cluster. Standalone
  `mongod` instances reject sessions with "Transaction numbers are only allowed
  on a replica set member or mongos."

## Auto-increment sequences

```go
nextID, err := engine.NextSequence(ctx, "order_id")
```

- Uses `FindOneAndUpdate` with `$inc: {value: 1}`, upsert, and
  `ReturnDocument: After` on the collection named `counters` (hard-coded)
- Each sequence name is a separate document keyed by `_id: name`
- First call for an unseen name returns `1`
- Returns an error for an empty `name` or a nil/uninitialized engine

## Logging

Package-level loggers with ANSI color prefixes and a level switch. Defined as
method-value assignments in `log.go`:

```go
var (
    Error  = errorLogger.Println   // func(...any)
    Errorf = errorLogger.Printf    // func(string, ...any)
    Info   = infoLogger.Println
    Infof  = infoLogger.Printf
)

const (
    InfoLevel = iota   // 0 - everything
    ErrorLevel         // 1 - error only
    Disabled           // 2 - silent
)

func SetLevel(level int)
```

Notes:

- Higher constant value means quieter; `SetLevel(Disabled)` silences both
  loggers
- The framework itself does not currently emit any log lines; these are
  exported for downstream code or future instrumentation
- Output goes to `os.Stdout`. The ANSI color codes will appear as literal
  escape sequences in terminals that do not support color
- To redirect, reassign the package-level vars (e.g. `lark_orm.Info =
myLogger.Println`); there is no interface-based logger injection

## Errors

Exported sentinel:

| Sentinel                | Source          | Raised when                                             |
| ----------------------- | --------------- | ------------------------------------------------------- |
| `ErrCollectionRequired` | `query_exec.go` | Any execution method runs on a Query with no collection |

Plain `errors.New` strings (NOT comparable with `errors.Is`; match on substring
only if you must):

```
mongo uri is required
mongo database is required
engine is not initialized
sequence name is required
at least one document is required
collection is required before query execution    // also wrapped by ErrCollectionRequired
```

Driver errors from `mongo-driver` are returned as-is, not wrapped with `%w`.
Use the driver's own error helpers (`mongo.IsDuplicateKeyError`, comparison
against `mongo.ErrNoDocuments`, etc.) to classify them.

## Typical usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hangtiancheng/lark-go/lark_orm"
    "go.mongodb.org/mongo-driver/bson"
)

type User struct {
    ID        int64     `bson:"_id"`
    Name      string    `bson:"name"`
    Email     string    `bson:"email"`
    Age       int       `bson:"age"`
    CreatedAt time.Time `bson:"created_at"`
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    engine, err := lark_orm.NewEngine(ctx, "mongodb://localhost:27017", "mydb")
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close(ctx)

    _, err = engine.Collection("users").Insert(ctx,
        bson.M{"_id": 1, "name": "Alice", "age": 30},
        bson.M{"_id": 2, "name": "Bob",   "age": 25},
    )
    if err != nil {
        log.Fatal(err)
    }

    var users []User
    if err := engine.Model(&User{}).
        Where("age", ">=", 18).
        WhereNotNull("email").
        OrderBy("created_at", "desc").
        Limit(10).
        Select("name", "email").
        Find(ctx, &users); err != nil {
        log.Fatal(err)
    }

    avg, _ := engine.Collection("orders").
        Where("status", "completed").
        Avg(ctx, "total")
    _ = avg

    n, _ := engine.Model(&User{}).Where("active", true).Count(ctx)
    _ = n
}
```

## Do / Don't

Do build a fresh Query for each request rather than reusing one across calls:

```go
// Good: each branch builds its own query
adults := engine.Model(&User{}).Where("age", ">=", 18)
minors := engine.Model(&User{}).Where("age", "<",  18)
```

```go
// Bad: branches alias the same underlying slices and may interfere.
base   := engine.Model(&User{}).Where("active", true)
adults := base.Where("age", ">=", 18) // mutates base
minors := base.Where("age", "<",  18) // also mutates base
```

Do guard `Update` / `Delete` with at least one condition:

```go
// Good
_, err := engine.Collection("users").
    Where("_id", id).
    Delete(ctx)
```

```go
// Bad: empty filter chain deletes every document in "users".
_, err := engine.Collection("users").Delete(ctx)
```

Do pass `sc` inside transactions:

```go
// Good
err := engine.Transaction(ctx, func(sc context.Context, tx *lark_orm.Engine) error {
    _, err := tx.Collection("orders").Insert(sc, order)
    return err
})
```

```go
// Bad: using outer ctx silently skips the session; no transaction occurs.
err := engine.Transaction(ctx, func(sc context.Context, tx *lark_orm.Engine) error {
    _, err := tx.Collection("orders").Insert(ctx, order)
    return err
})
```

Do use `bson.M` literals when you want the `$set` auto-wrap:

```go
// Good: auto-wrapped to {$set: {name: "Bob"}}
engine.Collection("users").Where("_id", 1).Update(ctx, bson.M{"name": "Bob"})
```

```go
// Bad: bson.D bypasses auto-wrap; driver rejects the document.
engine.Collection("users").Where("_id", 1).Update(ctx, bson.D{{Key: "name", Value: "Bob"}})
```

## Pitfalls

These behaviors are real and are not obvious from reading method signatures.
Always factor them into generated code.

1. Empty filter on `Update` / `Delete` operates on the entire collection.
   There is no built-in guard.
2. `OrWhere` without a preceding `Where*` produces `{$or: [{}, ...]}`. Since
   `{}` matches every document in MongoDB, this degenerates into a full-table
   match. Always pair `OrWhere` with at least one `Where`.
3. `mergeFieldOp` overwrites silently when the existing field value is a
   non-map scalar. After `Where("age", 18).Where("age", ">", 10)` the final
   filter is `{age: {$gt: 10}}`; the equality is lost.
4. `Query` is a stateful mutable struct. Branching from a shared base via
   chained `Where*` calls aliases the underlying condition slice and can yield
   inconsistent filters. Start each query from `Collection`/`Model`.
5. `Pluck` mutates `q.fields`. Subsequent uses of the same Query inherit the
   single-field projection. Treat `Pluck` as terminal.
6. `Update` auto-wraps `$set` only for `bson.M` without `$`-prefixed keys.
   `bson.D`, `map[string]interface{}`, and structs bypass the wrap.
7. `Select` only supports inclusive projection. Repeated calls replace.
8. `OrderBy` direction matches `"desc"` or `"DESC"` only. Mixed casings like
   `"Desc"` silently fall back to ascending.
9. Numeric aggregators always return `float64`. For integer fields you lose
   the original type; for non-numeric fields the cursor decode fails.
10. Transactions require a replica set or sharded cluster.
11. `NextSequence` uses the hard-coded `counters` collection name; this is not
    configurable.

## File map

| File                 | Purpose                                                                          |
| -------------------- | -------------------------------------------------------------------------------- |
| `lark_orm.go`        | Package declaration only.                                                        |
| `engine.go`          | `Engine` type, connection, `Collection`, `Model`, `Transaction`, `NextSequence`. |
| `query.go`           | `Query` type and chainable condition / sort / limit / offset / select methods.   |
| `query_exec.go`      | Execution methods plus `ErrCollectionRequired` and `normalizeUpdate`.            |
| `query_aggregate.go` | Aggregation methods `Sum`, `Avg`, `Min`, `Max`, `Distinct`, `Pluck`.             |
| `filter.go`          | Condition chain to `bson.M` translation, projection builder.                     |
| `naming.go`          | `CollectionName`, `toSnake`, `pluralize`.                                        |
| `log.go`             | Colored loggers and `SetLevel`.                                                  |
| `lark_orm_test.go`   | Unit and integration tests; requires a reachable MongoDB.                        |

## Dependencies

- Go 1.26 or newer
- `go.mongodb.org/mongo-driver` v1.17+

## Stable vs. evolving surface

Stable (changes here will be treated as breaking):

- `NewEngine`, `Engine.Close`, `Engine.Collection`, `Engine.Model`,
  `Engine.Transaction`, `Engine.NextSequence`
- Query lifecycle: `Where`, `WhereIn`, `WhereNotIn`, `WhereNull`,
  `WhereNotNull`, `WhereBetween`, `OrderBy`, `Limit`, `Offset`, `Select`,
  `Insert`, `First`, `Find`, `Update`, `Delete`, `Count`, `Exists`,
  `EnsureIndexes`, `DropCollection`
- `ErrCollectionRequired`
- `CollectionName`

Evolving (signatures or semantics may shift; pin behavior with tests if you
depend on them):

- Numeric aggregator return type (`float64`) and zero-on-empty semantics
- `Pluck` side effect on `q.fields`
- Logger globals as method-value vars
- `counters` collection name used by `NextSequence`
- Plural / snake-case rules for irregular and acronym-heavy struct names
