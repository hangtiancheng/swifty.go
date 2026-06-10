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
  Engine lifecycle, or replacing knex+mysql usage with lark_orm+mongodb. SKIP
  for GORM, sqlx, ent, raw mongo-driver code that does not import lark_orm,
  or any non-MongoDB datastore.
---

# lark_orm

A Knex-inspired, chainable query builder ORM for MongoDB in Go. Provides a
fluent API for CRUD operations, aggregation, transactions, and auto-incrementing
sequences on top of the official `go.mongodb.org/mongo-driver`. The package
exports a single entry point (`NewEngine`) that yields an `Engine` handle from
which all queries originate via `Collection(name)` or `Model(struct)`.

Module path: `github.com/hangtiancheng/lark-go/lark_orm`

Source root: `lark_orm/`

Go toolchain: 1.26+

All exported types live directly in the `lark_orm` package (flat layout, no
sub-packages).

## Architecture overview

```
Engine (engine.go)
  |-- *mongo.Client            (lifecycle: connect, ping, disconnect)
  |-- *mongo.Database          (selected at construction time)
  |-- mongo.Session            (set only inside Transaction callback)
  |
  |-- Client() / Database() / DatabaseName()   [accessors, nil-safe]
  |-- Close(ctx) / DropDatabase(ctx)           [lifecycle]
  |-- Collection(name) -> *Query               [entry to query builder]
  |-- Model(value)     -> *Query               [auto-derives collection name]
  |-- Transaction(ctx, fn)                     [session-scoped sub-Engine]
  |-- NextSequence(ctx, name)                  [atomic counter, "counters" col]

Query (query.go + query_exec.go + query_aggregate.go, mutable, chainable)
  |-- collection  *mongo.Collection
  |-- engine      *Engine
  |-- conditions  []condition       <- Where / WhereIn / WhereNotIn / etc.
  |-- orGroups    [][]condition      <- OrWhere
  |-- sort        bson.D            <- OrderBy
  |-- limit       int64             <- Limit
  |-- skip        int64             <- Offset
  |-- fields      []string          <- Select
  |
  |-- [conditions] Where / WhereIn / WhereNotIn / WhereNull / WhereNotNull
  |                WhereBetween / OrWhere
  |-- [modifiers]  OrderBy / Limit / Offset / Select
  |-- [execution]  Insert / First / Find / Update / Delete / Count / Exists
  |                EnsureIndexes / DropCollection
  |-- [aggregate]  Sum / Avg / Min / Max / Distinct / Pluck

Filter builder (filter.go)
  |-- buildFilter()      -> bson.M   [condition chain -> MongoDB query doc]
  |-- buildProjection()  -> bson.M   [fields -> inclusive projection or nil]

Naming (naming.go)
  |-- CollectionName(value) string   [struct type -> snake_case plural name]

Logging (log.go)
  |-- Error / Errorf / Info / Infof  [package-level method-value vars]
  |-- SetLevel(int)                  [InfoLevel / ErrorLevel / Disabled]
```

## Core types

### Engine

```go
type Engine struct {
    // unexported: client, database, databaseName, session
}
```

| Symbol       | Signature                                                                                                | Description                                                                                                                  |
| ------------ | -------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| NewEngine    | `func NewEngine(ctx context.Context, uri string, database string) (*Engine, error)`                      | Connect, ping, return ready engine. Errors on empty URI, empty database, or connectivity failure.                            |
| Client       | `func (e *Engine) Client() *mongo.Client`                                                                | Underlying driver client. Nil-safe.                                                                                          |
| Database     | `func (e *Engine) Database() *mongo.Database`                                                            | Active database handle. Nil-safe.                                                                                            |
| DatabaseName | `func (e *Engine) DatabaseName() string`                                                                 | Database name string. Nil-safe.                                                                                              |
| Collection   | `func (e *Engine) Collection(name string) *Query`                                                        | Start a query on the named collection. Empty/whitespace name yields a Query whose exec methods return ErrCollectionRequired. |
| Model        | `func (e *Engine) Model(value interface{}) *Query`                                                       | Equivalent to `Collection(CollectionName(value))`.                                                                           |
| Close        | `func (e *Engine) Close(ctx context.Context) error`                                                      | Disconnect the client. Nil-safe.                                                                                             |
| DropDatabase | `func (e *Engine) DropDatabase(ctx context.Context) error`                                               | Drop the entire database. Nil-safe.                                                                                          |
| NextSequence | `func (e *Engine) NextSequence(ctx context.Context, name string) (int64, error)`                         | Atomic counter via FindOneAndUpdate on hard-coded `counters` collection. First call returns 1.                               |
| Transaction  | `func (e *Engine) Transaction(ctx context.Context, fn func(sc context.Context, tx *Engine) error) error` | Session transaction. The callback receives a session context and a sub-Engine.                                               |

### Query

```go
type Query struct {
    // unexported: collection, engine, conditions, orGroups, sort, limit, skip, fields
}
```

Condition methods (return `*Query` for chaining):

| Method       | Signature                                                                              | Behavior                                                             |
| ------------ | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Where        | `func (q *Query) Where(args ...interface{}) *Query`                                    | 2-arg: equality. 3-arg: `(field, op, value)`.                        |
| WhereIn      | `func (q *Query) WhereIn(field string, values interface{}) *Query`                     | `$in` operator.                                                      |
| WhereNotIn   | `func (q *Query) WhereNotIn(field string, values interface{}) *Query`                  | `$nin` operator.                                                     |
| WhereNull    | `func (q *Query) WhereNull(field string) *Query`                                       | Sets `field: nil` in filter.                                         |
| WhereNotNull | `func (q *Query) WhereNotNull(field string) *Query`                                    | Sets `field: {$ne: nil}`.                                            |
| WhereBetween | `func (q *Query) WhereBetween(field string, low interface{}, high interface{}) *Query` | Sets `{$gte: low, $lte: high}`, merges with other ops on same field. |
| OrWhere      | `func (q *Query) OrWhere(args ...interface{}) *Query`                                  | Each call appends a separate `$or` branch.                           |

Modifier methods (return `*Query` for chaining):

| Method  | Signature                                                           | Behavior                                                         |
| ------- | ------------------------------------------------------------------- | ---------------------------------------------------------------- |
| OrderBy | `func (q *Query) OrderBy(field string, direction ...string) *Query` | `"desc"` or `"DESC"` for descending; anything else is ascending. |
| Limit   | `func (q *Query) Limit(n int64) *Query`                             | Applied only when n > 0.                                         |
| Offset  | `func (q *Query) Offset(n int64) *Query`                            | Applied only when n > 0. Works with both Find and First.         |
| Select  | `func (q *Query) Select(fields ...string) *Query`                   | Inclusive projection. Repeated calls replace, not merge.         |

Execution methods:

| Method         | Signature                                                                                          | Behavior                                                                                             |
| -------------- | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Insert         | `func (q *Query) Insert(ctx context.Context, documents ...interface{}) (InsertResult, error)`      | 1 doc uses InsertOne; 2+ docs use InsertMany. Requires at least one doc.                             |
| First          | `func (q *Query) First(ctx context.Context, out interface{}) error`                                | FindOne. Honors sort, skip, projection. Returns `mongo.ErrNoDocuments` when nothing matches.         |
| Find           | `func (q *Query) Find(ctx context.Context, out interface{}) error`                                 | Find + cursor.All. Loads entire result set into `out`.                                               |
| Update         | `func (q *Query) Update(ctx context.Context, update interface{}) (int64, error)`                   | UpdateMany. Returns ModifiedCount. Auto-wraps `bson.M` without `$`-prefixed keys into `{$set: ...}`. |
| Delete         | `func (q *Query) Delete(ctx context.Context) (int64, error)`                                       | DeleteMany. Returns DeletedCount.                                                                    |
| Count          | `func (q *Query) Count(ctx context.Context) (int64, error)`                                        | CountDocuments.                                                                                      |
| Exists         | `func (q *Query) Exists(ctx context.Context) (bool, error)`                                        | `Count(ctx) > 0`.                                                                                    |
| EnsureIndexes  | `func (q *Query) EnsureIndexes(ctx context.Context, indexes []mongo.IndexModel) ([]string, error)` | No-op when slice is empty.                                                                           |
| DropCollection | `func (q *Query) DropCollection(ctx context.Context) error`                                        | Drops the underlying collection.                                                                     |

Aggregation methods:

| Method   | Signature                                                                            | Behavior                                                              |
| -------- | ------------------------------------------------------------------------------------ | --------------------------------------------------------------------- |
| Sum      | `func (q *Query) Sum(ctx context.Context, field string) (float64, error)`            | `$sum`. Returns `(0, nil)` when no documents match.                   |
| Avg      | `func (q *Query) Avg(ctx context.Context, field string) (float64, error)`            | `$avg`. Returns `(0, nil)` when no documents match.                   |
| Min      | `func (q *Query) Min(ctx context.Context, field string) (float64, error)`            | `$min`. Non-numeric fields fail at decode.                            |
| Max      | `func (q *Query) Max(ctx context.Context, field string) (float64, error)`            | `$max`. Non-numeric fields fail at decode.                            |
| Distinct | `func (q *Query) Distinct(ctx context.Context, field string) ([]interface{}, error)` | Direct call to Collection.Distinct. Caller must type-assert elements. |
| Pluck    | `func (q *Query) Pluck(ctx context.Context, field string, out interface{}) error`    | Mutates `q.fields` then calls Find. Treat as terminal.                |

### InsertResult

```go
type InsertResult struct {
    InsertedIDs   []interface{}
    InsertedCount int64
}
```

### ErrCollectionRequired

```go
var ErrCollectionRequired = errors.New("collection is required before query execution")
```

Returned by every execution method when the Query has a nil collection (caused
by calling `engine.Collection("")` or operating on a nil Query).

### CollectionName

```go
func CollectionName(value interface{}) string
```

Derives a collection name from a struct type via reflection: unwrap pointers,
unwrap slice/array element, require struct kind, convert CamelCase to
snake_case, then pluralize.

### Log constants and functions

```go
const (
    InfoLevel  = iota  // 0
    ErrorLevel         // 1
    Disabled           // 2
)

var (
    Error  = errorLogger.Println
    Errorf = errorLogger.Printf
    Info   = infoLogger.Println
    Infof  = infoLogger.Printf
)

func SetLevel(level int)
```

Comparison operators recognized in `Where(field, op, value)`:

```
=  !=  <>  >  >=  <  <=  $in  $nin
```

Any operator string not in the map passes through to MongoDB verbatim (e.g.,
`"$regex"`, `"$exists"`).

## Internal implementation details affecting correctness

### Filter construction (filter.go)

`buildFilter()` translates the condition chain into a `bson.M`:

- Equality (`op == "="`) sets `field: value` directly on the map.
- `op == "null"` sets `field: nil`.
- `op == "notNull"` merges `{$ne: nil}` via `mergeFieldOp`.
- `op == "between"` type-asserts the value to `[2]interface{}` and merges both
  `$gte` and `$lte` onto the field.
- All other ops are looked up in `opMap` (or passed through if absent), then
  merged via `mergeFieldOp`.

`mergeFieldOp` accumulates multiple operators on the same field into a single
`bson.M` sub-document. If the existing value for a field is already a `bson.M`,
it adds the new operator key. If the existing value is a non-map scalar (from a
prior equality condition), the scalar is overwritten with a new `bson.M`
containing only the new operator -- the equality is silently lost.

When `orGroups` is non-empty, the entire output becomes
`{$or: [<base-filter>, <group1>, <group2>, ...]}` where `<base-filter>` is
built from the main `conditions` slice.

`buildProjection()` returns `nil` when `q.fields` is empty, otherwise a
`bson.M` mapping each field to `1` (inclusive projection only; `_id` is not
explicitly excluded).

### Update normalization (query_exec.go)

`normalizeUpdate` checks whether the argument is the concrete type `bson.M`. If
so, it scans for any key starting with `$`. If none found, it wraps the
document: `bson.M{"$set": originalDoc}`. If the argument is any other type
(`bson.D`, `map[string]interface{}`, a struct), it passes through unchanged and
the driver will reject documents that lack an update operator.

### Naming (naming.go)

`toSnake` inserts an underscore before every uppercase letter that is not at
position 0. Acronyms are split letter-by-letter (`HTTPServer` becomes
`h_t_t_p_server`).

Pluralization rules applied in order:

1. Ends in consonant + `y`: replace `y` with `ies`
2. Ends in `s`, `x`, `sh`, or `ch`: append `es`
3. Otherwise: append `s`

Irregular plurals and already-plural words are not handled.

### Transaction session propagation (engine.go)

`Transaction` creates a new `Engine` value (`txEngine`) that shares the same
client, database, and database name, but carries the session. The session is
private and not exposed to query execution methods -- the transactional
guarantee depends entirely on the caller passing the `mongo.SessionContext` (the
`sc` parameter) into every lark_orm method call inside the callback. If the
outer `ctx` is passed instead, operations silently run outside the transaction.

### Query mutability

`Query` methods append to internal slices and return the same pointer receiver.
The struct is not copied. Branching from a shared Query aliases the underlying
slices; subsequent mutations affect all branches.

## Typical usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/hangtiancheng/lark-go/lark_orm"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
    ID        int64     `bson:"_id"`
    Name      string    `bson:"name"`
    Email     string    `bson:"email"`
    Age       int       `bson:"age"`
    CreatedAt time.Time `bson:"created_at"`
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    engine, err := lark_orm.NewEngine(ctx, "mongodb://localhost:27017", "myapp")
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close(ctx)

    // Insert documents
    result, err := engine.Collection("users").Insert(ctx,
        &User{ID: 1, Name: "Alice", Email: "alice@example.com", Age: 30, CreatedAt: time.Now()},
        &User{ID: 2, Name: "Bob", Email: "bob@example.com", Age: 25, CreatedAt: time.Now()},
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("inserted:", result.InsertedCount)

    // Query with conditions, ordering, and projection
    var users []User
    err = engine.Model(&User{}).
        Where("age", ">=", 18).
        WhereNotNull("email").
        OrderBy("created_at", "desc").
        Limit(10).
        Offset(0).
        Select("name", "email", "_id").
        Find(ctx, &users)
    if err != nil {
        log.Fatal(err)
    }

    // Single document lookup
    var user User
    err = engine.Model(&User{}).
        Where("_id", 1).
        First(ctx, &user)
    if err != nil {
        log.Fatal(err)
    }

    // Update with auto $set wrap
    modified, err := engine.Collection("users").
        Where("_id", 1).
        Update(ctx, bson.M{"name": "Alice Updated"})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("modified:", modified)

    // Update with explicit operator (no $set wrap)
    _, _ = engine.Collection("users").
        Where("_id", 1).
        Update(ctx, bson.M{"$inc": bson.M{"age": 1}})

    // Delete with condition
    deleted, _ := engine.Collection("users").
        Where("age", "<", 18).
        Delete(ctx)
    fmt.Println("deleted:", deleted)

    // Aggregation
    avg, _ := engine.Collection("users").
        Where("age", ">", 0).
        Avg(ctx, "age")
    fmt.Println("average age:", avg)

    // Count and Exists
    count, _ := engine.Model(&User{}).Count(ctx)
    exists, _ := engine.Model(&User{}).Where("name", "Alice Updated").Exists(ctx)
    fmt.Println("count:", count, "exists:", exists)

    // Auto-increment sequence
    nextID, _ := engine.NextSequence(ctx, "user_id")
    fmt.Println("next id:", nextID)

    // Transaction (requires replica set)
    err = engine.Transaction(ctx, func(sc context.Context, tx *lark_orm.Engine) error {
        _, err := tx.Collection("users").
            Where("_id", 1).
            Update(sc, bson.M{"$inc": bson.M{"age": -1}})
        if err != nil {
            return err
        }
        _, err = tx.Collection("users").
            Where("_id", 2).
            Update(sc, bson.M{"$inc": bson.M{"age": 1}})
        return err
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create indexes
    _, err = engine.Collection("users").EnsureIndexes(ctx, []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "email", Value: 1}},
            Options: options.Index().SetUnique(true).SetName("uniq_email"),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

## Pitfalls / known limitations

1. Empty filter on `Update` or `Delete` operates on the entire collection.
   There is no built-in guard against unconditional mass mutation.
2. `OrWhere` without a preceding `Where` produces `{$or: [{}, ...]}`. The empty
   document `{}` matches all documents in MongoDB, making the query degenerate
   to a full-collection match.
3. `mergeFieldOp` silently overwrites a prior equality value when a subsequent
   operator condition targets the same field. After
   `Where("age", 18).Where("age", ">", 10)` the final filter is
   `{age: {$gt: 10}}` and the equality is lost.
4. `Query` is a mutable struct. Branching from a shared base via chained
   `Where` calls aliases the underlying condition slice and produces
   inconsistent filters. Always start each query from `Collection` or `Model`.
5. `Pluck` mutates `q.fields` as a side effect. Subsequent uses of the same
   Query inherit the single-field projection. Treat `Pluck` as terminal.
6. `Update` auto-wraps `$set` only for the concrete type `bson.M` without any
   `$`-prefixed keys. `bson.D`, `map[string]interface{}`, and structs bypass
   the wrap and the driver rejects documents lacking an update operator.
7. `Select` supports inclusive projection only. MongoDB does not allow mixing
   inclusion and exclusion (except for `_id`). Repeated calls replace.
8. `OrderBy` direction matching is strict: only `"desc"` and `"DESC"` produce
   descending order. Mixed casings like `"Desc"` or `"DESC "` silently fall
   back to ascending.
9. Numeric aggregators (`Sum`, `Avg`, `Min`, `Max`) always return `float64`.
   For integer fields the original type is lost; for non-numeric fields the
   cursor decode will fail with an error.
10. Transactions require a MongoDB replica set or sharded cluster. Standalone
    `mongod` instances reject sessions with a "Transaction numbers are only
    allowed on a replica set member or mongos" error.
11. `NextSequence` uses the hard-coded collection name `counters`. This is not
    configurable and will conflict with application code that uses a collection
    of the same name for other purposes.
12. `CollectionName` splits acronyms letter-by-letter (`HTTPServer` becomes
    `h_t_t_p_servers`). For acronym-heavy names, use
    `engine.Collection("explicit_name")` instead of `engine.Model(...)`.
13. Irregular plurals are not handled (`Person` becomes `persons`, `Child`
    becomes `childs`). Override with `engine.Collection(...)`.
14. There is no `CollectionNamer` interface; the only way to override the
    derived name is to call `Collection` directly.
15. The `session` field on `txEngine` is set but never read by Query execution
    methods -- transactional behavior depends solely on passing the
    `mongo.SessionContext` as `ctx`. The session field is present for potential
    future use.

## File map

| File                 | Purpose                                                                                                                                            |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `lark_orm.go`        | Package declaration only (anchor file).                                                                                                            |
| `engine.go`          | `Engine` struct, `NewEngine`, `Client`, `Database`, `DatabaseName`, `Collection`, `Model`, `Close`, `DropDatabase`, `NextSequence`, `Transaction`. |
| `query.go`           | `Query` struct and chainable condition/sort/limit/offset/select methods.                                                                           |
| `query_exec.go`      | Execution methods, `InsertResult`, `ErrCollectionRequired`, `normalizeUpdate`.                                                                     |
| `query_aggregate.go` | `Sum`, `Avg`, `Min`, `Max`, `Distinct`, `Pluck`, private `aggregate` helper.                                                                       |
| `filter.go`          | `condition` type, `parseWhere`, `opMap`, `buildFilter`, `applyCondition`, `mergeFieldOp`, `buildProjection`.                                       |
| `naming.go`          | `CollectionName`, `toSnake`, `pluralize`, `isVowel`.                                                                                               |
| `log.go`             | `Error`, `Errorf`, `Info`, `Infof`, `InfoLevel`, `ErrorLevel`, `Disabled`, `SetLevel`.                                                             |
| `lark_orm_test.go`   | Unit and integration tests; requires a reachable MongoDB instance.                                                                                 |
| `go.mod`             | Module declaration and dependencies.                                                                                                               |

## Dependencies

- Go 1.26 or newer (per `go.mod`)
- `go.mongodb.org/mongo-driver` v1.17.6

Transitive dependencies (indirect): `golang.org/x/crypto`, `golang.org/x/sync`,
`golang.org/x/text`, `github.com/klauspost/compress`,
`github.com/golang/snappy`, `github.com/montanaflynn/stats`,
`github.com/xdg-go/pbkdf2`, `github.com/xdg-go/scram`,
`github.com/xdg-go/stringprep`, `github.com/youmark/pkcs8`.

## Cross-references to sibling skills

- `lark-cache`: Distributed cache framework. When building an application that
  caches MongoDB query results, combine lark_orm for persistence with lark_cache
  for distributed read-through caching and invalidation.
- `lark-http`: HTTP server framework. Use lark_http to expose REST endpoints
  that perform database operations via lark_orm Engine instances. The Engine is
  typically initialized at server startup and injected into handlers.
- `lark-rpc`: gRPC service framework. Use lark_rpc when database-backed services
  communicate over gRPC; the Engine provides the persistence layer behind RPC
  method implementations.
