---
name: swifty-orm
description: >
  Knex-style chainable MongoDB ORM for Go (module
  github.com/hangtiancheng/swifty.go/swifty_orm). Use when Go code calls
  swifty_orm.NewEngine, engine.Collection, engine.Model, the chainable Query
  builder (Where equality/operator/object forms, WhereNot, WhereIn,
  WhereNotIn, WhereNull, WhereNotNull, WhereBetween, WhereLike, WhereILike,
  OrWhere and Or-variants, OrderBy, Limit, Offset, Select, Clone), terminal
  methods (Insert, First, Find, Update, Upsert, Increment, Decrement, Delete,
  Count, Exists, EnsureIndexes, DropCollection), aggregation (Sum, Avg, Min,
  Max, Distinct, CountDistinct, Pluck), grouped aggregation (GroupBy, Having,
  CountAs, SumAs, AvgAs, MinAs, MaxAs, Aggregate), streaming (Cursor, Each),
  Transaction with auto session binding, NextSequence, CollectionName,
  ErrNotFound, ErrCollectionRequired, or any import of the module. Also use
  for Knex-style query chaining over MongoDB in Go. Do NOT use for GORM,
  sqlx, ent, lark_orm, raw mongo-driver code without swifty_orm, or any
  non-MongoDB datastore.
---

# swifty_orm

A Knex-inspired, chainable query builder ORM for MongoDB in Go, built on the
official `go.mongodb.org/mongo-driver`. The design philosophy is a faithful
mapping of Knex.js query semantics onto MongoDB: all conditions on the same
field AND-combine without silent overwrites, invalid builder input is recorded
and surfaced as an error at execution time (never a panic), plain update
documents are auto-wrapped in `$set`, and `Update` returns the matched count
(Knex "affected rows" semantics). The package exposes two core abstractions:
`Engine` (connection, transaction, and sequence management) and `Query`
(mutable chainable builder with terminal, aggregation, grouping, and streaming
methods). Flat layout, no sub-packages.

Module path: `github.com/hangtiancheng/swifty.go/swifty_orm`

Source root: `swifty_orm/`

Go toolchain: 1.26.0+

## Architecture overview

```
Engine (engine.go)
  |-- client       *mongo.Client     (connect + ping at construction)
  |-- database     *mongo.Database
  |-- databaseName string
  |-- session      mongo.Session     (set only on Transaction sub-Engine)
  |
  |-- Client() / Database() / DatabaseName()    [nil-safe accessors]
  |-- Close(ctx) / DropDatabase(ctx)            [lifecycle]
  |-- Collection(name) -> *Query                [entry to query builder]
  |-- Model(value)     -> *Query                [Collection(CollectionName(v))]
  |-- Transaction(ctx, fn)                      [session-scoped sub-Engine]
  |-- NextSequence(ctx, name)                   ["counters" collection, $inc]
  |-- sessionContext(ctx)                       [auto-binds tx session to ctx]

Query (query.go; mutable, chainable; owns builder state + first error)
  |-- collection   *mongo.Collection
  |-- engine       *Engine            (for session binding via execCtx)
  |-- conditions   []condition        <- Where family (AND chain)
  |-- orGroups     [][]condition      <- OrWhere family ($or branches)
  |-- sort         bson.D             <- OrderBy
  |-- limit, skip  int64              <- Limit / Offset
  |-- fields       []string           <- Select ("-" prefix = exclude)
  |-- groupFields  []string           <- GroupBy
  |-- havingConds  []condition        <- Having
  |-- aggSpecs     []aggSpec          <- CountAs / SumAs / AvgAs / MinAs / MaxAs
  |-- err          error              <- first builder error, surfaced at exec
  |
  |-- [exec: query_exec.go]      Insert / First / Find / Update / Upsert /
  |                              Increment / Decrement / Delete / Count /
  |                              Exists / EnsureIndexes / DropCollection
  |-- [aggregate: query_aggregate.go]  Sum / Avg / Min / Max / Distinct /
  |                                    CountDistinct / Pluck
  |-- [group: query_group.go]    GroupBy / Having / *As aliases / Aggregate
  |-- [stream: query_stream.go]  Cursor / Each -> *Cursor
  |-- Clone()                    deep copy of all builder state

Query pipeline at execution time:
  preflight (collection bound? builder err? pending group state?)
    -> buildFilter (conditions + orGroups -> bson.M)
    -> buildProjection / findOptions (sort, limit, skip, projection)
    -> execCtx (bind transaction session if any)
    -> mongo-driver call

Grouped aggregation pipeline (Aggregate):
  $match(where) -> $group -> $project -> $match(having) -> $sort -> $skip -> $limit

Cursor (query_stream.go)
  |-- cursor *mongo.Cursor
  |-- engine *Engine        (keeps getMore/killCursors on the tx session)
  |-- Next / Decode / Current / Err / Close

Filter builder (filter.go)
  |-- parseWhere / parseWhereMap / normalizeOp / toBetweenPair / likeToRegex
  |-- buildFilter / buildConditionFilter   [condition chain -> bson.M, $and fallback]
  |-- buildProjection                      [fields -> inclusive/exclusive projection]

Naming (naming.go):   CollectionName(value) -> snake_case plural collection name
Logging (log.go):     Error / Errorf / Info / Infof, SetLevel, level constants
```

## Core types

### Engine

```go
type Engine struct {
    // unexported: client, database, databaseName, session
}
```

| Symbol       | Signature                                                                                                 | Behavior                                                                                                                                              |
| ------------ | --------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| NewEngine    | `func NewEngine(ctx context.Context, uri string, database string) (*Engine, error)`                       | Connects and pings. Errors on empty/whitespace URI or database name, or connectivity failure. On ping failure the client is disconnected before return. |
| Client       | `func (e *Engine) Client() *mongo.Client`                                                                  | Underlying driver client. Nil-receiver safe (returns nil).                                                                                              |
| Database     | `func (e *Engine) Database() *mongo.Database`                                                              | Active database handle. Nil-receiver safe.                                                                                                              |
| DatabaseName | `func (e *Engine) DatabaseName() string`                                                                   | Database name. Nil-receiver safe (returns "").                                                                                                          |
| Collection   | `func (e *Engine) Collection(name string) *Query`                                                          | Starts a query. An empty/whitespace name (or nil engine/database) yields a Query whose execution methods return ErrCollectionRequired.                  |
| Model        | `func (e *Engine) Model(value interface{}) *Query`                                                         | Equivalent to `Collection(CollectionName(value))`.                                                                                                      |
| Close        | `func (e *Engine) Close(ctx context.Context) error`                                                        | Disconnects the client. Nil-safe no-op.                                                                                                                 |
| DropDatabase | `func (e *Engine) DropDatabase(ctx context.Context) error`                                                 | Drops the entire database. Nil-safe no-op.                                                                                                              |
| NextSequence | `func (e *Engine) NextSequence(ctx context.Context, name string) (int64, error)`                           | Atomic counter via FindOneAndUpdate ($inc, upsert, ReturnDocument After) on the hard-coded `counters` collection. First call returns 1. Errors on empty name or uninitialized engine. Joins an active transaction session. |
| Transaction  | `func (e *Engine) Transaction(ctx context.Context, fn func(sc context.Context, tx *Engine) error) error`  | Starts a session and runs fn inside `session.WithTransaction`. fn receives the session context and a sub-Engine that carries the session. Returning nil commits; returning an error aborts. |

### Query

All chainable methods mutate the receiver and return the same `*Query`
pointer. The first invalid builder input is recorded in an internal `err`
field and returned by the next execution method (no panics, no silently
broken filters).

Condition methods (main AND chain):

| Method          | Signature                                                                                | Behavior                                                                                                                                |
| --------------- | ----------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Where           | `func (q *Query) Where(args ...interface{}) *Query`                                       | 1-arg: `bson.M`/`map[string]interface{}`, one equality per key (keys processed in sorted order; nil value becomes a null check). 2-arg: equality (nil value becomes null check). 3-arg: `(field, op, value)`. Invalid args record a builder error. |
| WhereNot        | `func (q *Query) WhereNot(field string, value interface{}) *Query`                        | `{field: {$ne: value}}`.                                                                                                                    |
| WhereIn         | `func (q *Query) WhereIn(field string, values interface{}) *Query`                        | `$in`.                                                                                                                                      |
| WhereNotIn      | `func (q *Query) WhereNotIn(field string, values interface{}) *Query`                     | `$nin`.                                                                                                                                     |
| WhereNull       | `func (q *Query) WhereNull(field string) *Query`                                          | `{field: nil}`.                                                                                                                             |
| WhereNotNull    | `func (q *Query) WhereNotNull(field string) *Query`                                       | `{field: {$ne: nil}}`.                                                                                                                      |
| WhereBetween    | `func (q *Query) WhereBetween(field string, low, high interface{}) *Query`                | `{$gte: low, $lte: high}`.                                                                                                                  |
| WhereNotBetween | `func (q *Query) WhereNotBetween(field string, low, high interface{}) *Query`             | `{$not: {$gte: low, $lte: high}}`.                                                                                                          |
| WhereLike       | `func (q *Query) WhereLike(field string, pattern string) *Query`                          | SQL LIKE pattern (`%` any sequence, `_` one char) as an anchored, metacharacter-escaped `$regex`. Case-sensitive.                           |
| WhereILike      | `func (q *Query) WhereILike(field string, pattern string) *Query`                         | Same as WhereLike with regex option `i` (case-insensitive).                                                                                 |

Or-branch methods (each call appends one `$or` branch):

| Method          | Signature                                                                    | Behavior                                                                                                     |
| --------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| OrWhere         | `func (q *Query) OrWhere(args ...interface{}) *Query`                          | Same argument forms as Where. The object form makes a single branch whose keys AND-combine. An empty map is a no-op (an empty branch would match everything). |
| OrWhereNot      | `func (q *Query) OrWhereNot(field string, value interface{}) *Query`           | Branch with `$ne`.                                                                                                |
| OrWhereIn       | `func (q *Query) OrWhereIn(field string, values interface{}) *Query`           | Branch with `$in`.                                                                                                |
| OrWhereNotIn    | `func (q *Query) OrWhereNotIn(field string, values interface{}) *Query`        | Branch with `$nin`.                                                                                               |
| OrWhereNull     | `func (q *Query) OrWhereNull(field string) *Query`                             | Branch with null check.                                                                                           |
| OrWhereNotNull  | `func (q *Query) OrWhereNotNull(field string) *Query`                          | Branch with `{$ne: nil}`.                                                                                          |
| OrWhereBetween  | `func (q *Query) OrWhereBetween(field string, low, high interface{}) *Query`   | Branch with `{$gte, $lte}`.                                                                                        |

Modifier methods:

| Method  | Signature                                                            | Behavior                                                                                                     |
| ------- | ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| OrderBy | `func (q *Query) OrderBy(field string, direction ...string) *Query`    | Appends a sort key. Direction is trimmed and compared case-insensitively; "desc" descends, anything else ascends. |
| Limit   | `func (q *Query) Limit(n int64) *Query`                                | Applied only when n > 0.                                                                                           |
| Offset  | `func (q *Query) Offset(n int64) *Query`                               | Applied only when n > 0. Honored by Find, First, Cursor/Each, Pluck, and Aggregate.                                |
| Select  | `func (q *Query) Select(fields ...string) *Query`                      | Sets the projection, replacing any previous one. Inclusive by default; a "-" prefix excludes a field (e.g. `"-_id"`). |
| Clone   | `func (q *Query) Clone() *Query`                                       | Deep copy of all builder state (conditions, orGroups, sort, fields, group state, err). Nil-safe.                   |

Execution methods (query_exec.go):

| Method         | Signature                                                                                           | Behavior                                                                                                                                                    |
| -------------- | ----------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Insert         | `func (q *Query) Insert(ctx context.Context, documents ...interface{}) (InsertResult, error)`         | A single slice/array argument is expanded into individual documents (`bson.D` and `[]byte` excluded). 1 doc uses InsertOne, 2+ use InsertMany. On partial InsertMany failure, returns the successfully inserted IDs alongside the error. Errors when no documents remain. |
| First          | `func (q *Query) First(ctx context.Context, out interface{}) error`                                   | FindOne honoring sort, offset, and projection (Limit is irrelevant). Returns ErrNotFound (alias of mongo.ErrNoDocuments) when nothing matches.                  |
| Find           | `func (q *Query) Find(ctx context.Context, out interface{}) error`                                    | Find + cursor.All; loads the entire result set into out.                                                                                                        |
| Update         | `func (q *Query) Update(ctx context.Context, update interface{}) (int64, error)`                      | UpdateMany. Returns MatchedCount (Knex affected-rows semantics; includes matched-but-unchanged documents). Plain documents are auto-wrapped in `$set` (see normalizeUpdate). |
| Upsert         | `func (q *Query) Upsert(ctx context.Context, update interface{}) (UpsertResult, error)`               | UpdateMany with upsert enabled. Inserts a document derived from the filter equalities plus the update when nothing matches.                                     |
| Increment      | `func (q *Query) Increment(ctx context.Context, field string, amount ...int64) (int64, error)`        | `$inc` by amount (default 1). Returns matched count.                                                                                                            |
| Decrement      | `func (q *Query) Decrement(ctx context.Context, field string, amount ...int64) (int64, error)`        | `$inc` by -amount (default 1). Returns matched count.                                                                                                           |
| Delete         | `func (q *Query) Delete(ctx context.Context) (int64, error)`                                          | DeleteMany. Returns DeletedCount.                                                                                                                               |
| Count          | `func (q *Query) Count(ctx context.Context) (int64, error)`                                           | CountDocuments with the built filter.                                                                                                                           |
| Exists         | `func (q *Query) Exists(ctx context.Context) (bool, error)`                                           | `Count(ctx) > 0`.                                                                                                                                               |
| EnsureIndexes  | `func (q *Query) EnsureIndexes(ctx context.Context, indexes []mongo.IndexModel) ([]string, error)`    | Indexes().CreateMany. No-op when the slice is empty.                                                                                                            |
| DropCollection | `func (q *Query) DropCollection(ctx context.Context) error`                                           | Drops the underlying collection.                                                                                                                                |

Aggregation methods (query_aggregate.go):

| Method        | Signature                                                                              | Behavior                                                                                                                                                        |
| ------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Sum           | `func (q *Query) Sum(ctx context.Context, field string) (float64, error)`                | `[$match, $group {$sum}]` pipeline. Returns (0, nil) when no documents match.                                                                                        |
| Avg           | `func (q *Query) Avg(ctx context.Context, field string) (float64, error)`                | `$avg`. Returns (0, nil) when no documents match.                                                                                                                    |
| Min           | `func (q *Query) Min(ctx context.Context, field string) (float64, error)`                | `$min`. Non-numeric result values fail at decode into float64.                                                                                                       |
| Max           | `func (q *Query) Max(ctx context.Context, field string) (float64, error)`                | `$max`. Non-numeric result values fail at decode into float64.                                                                                                       |
| Distinct      | `func (q *Query) Distinct(ctx context.Context, field string) ([]interface{}, error)`     | Collection.Distinct with the built filter. Caller must type-assert elements.                                                                                         |
| CountDistinct | `func (q *Query) CountDistinct(ctx context.Context, field string) (int64, error)`        | `len(Distinct(...))`.                                                                                                                                                |
| Pluck         | `func (q *Query) Pluck(ctx context.Context, field string, out interface{}) error`        | Collects one field's values into out, which must be a non-nil pointer to a slice of the value type (e.g. `*[]string`, `*[]int64`). Honors sort, limit, offset. Supports dotted field paths. Documents missing the field contribute the element zero value. Does not mutate the Query's projection. |

Grouped aggregation methods (query_group.go):

| Method    | Signature                                                              | Behavior                                                                                                                                  |
| --------- | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| GroupBy   | `func (q *Query) GroupBy(fields ...string) *Query`                         | Adds group keys. Empty field names record a builder error. Each key appears in result rows under its own name; dotted paths are flattened with underscores (`addr.city` -> `addr_city`). |
| Having    | `func (q *Query) Having(args ...interface{}) *Query`                       | Filters grouped rows. Same argument forms as Where. Must reference result column names (flattened group keys or aliases).                       |
| CountAs   | `func (q *Query) CountAs(alias string) *Query`                             | Per-group document count (`{$sum: 1}`) under alias.                                                                                             |
| SumAs     | `func (q *Query) SumAs(field string, alias string) *Query`                 | Per-group `$sum` of field under alias.                                                                                                          |
| AvgAs     | `func (q *Query) AvgAs(field string, alias string) *Query`                 | Per-group `$avg`.                                                                                                                               |
| MinAs     | `func (q *Query) MinAs(field string, alias string) *Query`                 | Per-group `$min`.                                                                                                                               |
| MaxAs     | `func (q *Query) MaxAs(field string, alias string) *Query`                 | Per-group `$max`.                                                                                                                               |
| Aggregate | `func (q *Query) Aggregate(ctx context.Context, out interface{}) error`    | Builds and runs the group pipeline, decoding rows into out (pointer to slice). Result rows contain group keys and aliases as top-level fields; `_id` is projected away. |

Alias validation (recorded as builder errors): alias must be non-empty, must
not be `_id`, must not start with `$` or contain `.`. Pipeline validation
(returned by Aggregate): GroupBy is required; `Select` cannot be combined with
GroupBy; duplicate/colliding flattened group keys and aliases are rejected;
Having and OrderBy may only reference result columns.

Streaming methods (query_stream.go):

| Method | Signature                                                                    | Behavior                                                                                                          |
| ------ | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| Cursor | `func (q *Query) Cursor(ctx context.Context) (*Cursor, error)`                 | Executes Find and returns a streaming Cursor honoring filter, sort, limit, offset, and projection. Caller must Close it. |
| Each   | `func (q *Query) Each(ctx context.Context, fn func(c *Cursor) error) error`    | Streams every matching document through fn, stopping at (and returning) the first error. Closes the cursor automatically. Returns the cursor's iteration error otherwise. |

### Cursor

```go
type Cursor struct {
    // unexported: cursor *mongo.Cursor, engine *Engine
}
```

| Method  | Signature                                            | Behavior                                                                                     |
| ------- | ------------------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| Next    | `func (c *Cursor) Next(ctx context.Context) bool`      | Advances to the next document; false at end of stream or on error (check Err).                    |
| Decode  | `func (c *Cursor) Decode(out interface{}) error`       | Unmarshals the current document into out.                                                         |
| Current | `func (c *Cursor) Current() bson.Raw`                  | Raw BSON of the current document.                                                                 |
| Err     | `func (c *Cursor) Err() error`                         | Error that terminated iteration, if any.                                                          |
| Close   | `func (c *Cursor) Close(ctx context.Context) error`    | Releases the server-side cursor.                                                                  |

Next and Close bind the transaction session to ctx when the Cursor was opened
through a Transaction sub-Engine, keeping getMore/killCursors inside the
transaction.

### InsertResult

```go
type InsertResult struct {
    InsertedIDs   []interface{}
    InsertedCount int64
}
```

On partial InsertMany failure (e.g. a unique-index violation mid-batch),
`InsertedIDs`/`InsertedCount` reflect the documents that were inserted and the
error is returned alongside.

### UpsertResult

```go
type UpsertResult struct {
    MatchedCount  int64
    ModifiedCount int64
    UpsertedCount int64
    UpsertedID    interface{}
}
```

### Sentinel errors

```go
var ErrCollectionRequired = errors.New("collection is required before query execution")
var ErrNotFound = mongo.ErrNoDocuments // alias; errors.Is matches both
```

`ErrCollectionRequired` is returned by every execution method when the Query
has a nil collection (nil Query, `engine.Collection("")`, or an uninitialized
engine). `ErrNotFound` is returned by `First` when no document matches, so
callers need not import the driver.

### CollectionName

```go
func CollectionName(value interface{}) string
```

Derives a collection name via reflection: unwrap pointers, unwrap slice/array
element (and its pointers), require struct kind (otherwise return ""), convert
CamelCase to snake_case, then pluralize. `User` -> `users`, `ChatHistory` ->
`chat_histories`, `Category` -> `categories`, `Address` -> `addresses`.

### Logging

```go
const (
    InfoLevel  = iota // 0
    ErrorLevel        // 1
    Disabled          // 2
)

var (
    Error  = errorLogger.Println // stderr, red "[error]" prefix
    Errorf = errorLogger.Printf
    Info   = infoLogger.Println  // stdout, blue "[info ]" prefix
    Infof  = infoLogger.Printf
)

func SetLevel(level int)
```

`SetLevel` is mutex-guarded. It first resets outputs to their defaults (info
to stdout, error to stderr), then discards streams whose level is below the
requested one: `InfoLevel` enables everything, `ErrorLevel` keeps errors only,
`Disabled` suppresses all output. The `Error`/`Info` variables are method
values bound at package init; the loggers themselves are not replaceable.

## Internal implementation details affecting correctness

### Operator aliases (filter.go)

`Where(field, op, value)` operators are trimmed and lowercased, then resolved
through `opAliases`:

```
=  ==            -> equality
!=  <>           -> $ne
>  >=  <  <=     -> $gt  $gte  $lt  $lte
in               -> $in
not in / nin     -> $nin
like / ilike     -> anchored $regex (case-sensitive / -insensitive)
between          -> $gte + $lte      (value: any 2-element slice or array)
not between      -> $not {$gte,$lte}
```

Operators starting with `$` (e.g. `"$regex"`, `"$exists"`, `"$in"`) pass
through to MongoDB verbatim. Any other unrecognized operator records a builder
error; the query never reaches the server with a broken filter. `like`/`ilike`
require a string pattern; `between` requires a 2-element slice/array; both
record builder errors otherwise.

### Filter composition (filter.go)

`buildFilter` translates `conditions` into a `bson.M` via
`buildConditionFilter`, which guarantees AND semantics without silent loss:

- Equality sets `field: value`. If the field already holds an operator map
  created by the builder, the equality merges in as `$eq`; if the slot is
  occupied and cannot merge (duplicate equality, duplicate operator), the
  condition drops into a top-level `$and` array instead of overwriting.
- The builder distinguishes operator maps it created from user-supplied
  equality values that happen to be `bson.M` (sub-document equality); the
  latter are never mutated.
- `between` merges `$gte` and `$lte`; `notBetween` sets `$not: {$gte, $lte}`;
  `like`/`ilike` produce `primitive.Regex` values from `likeToRegex`, which
  escapes all regex metacharacters and maps `%` to `.*` and `_` to `.`,
  anchored with `^...$`.

Or-group composition: when `orGroups` is non-empty, the main-chain filter
becomes one branch (only if it has conditions -- an empty base branch is never
emitted, so an OrWhere-only query cannot degenerate to a match-all), each
group becomes another branch, and a single remaining branch is returned
unwrapped instead of `{$or: [...]}`. A `Where` added after an `OrWhere` still
joins the main AND chain: `Where(a).OrWhere(b).Where(c)` means `(a AND c) OR b`.

`buildProjection` returns nil when no fields are selected; otherwise each
field maps to 1, or 0 when prefixed with `-`.

### Builder error accumulation and preflight (query.go, query_exec.go)

Invalid builder input (wrong Where arity, non-string field, unknown operator,
bad between/like values, empty group field, invalid alias) is stored in
`Query.err` via `setErr` (first error wins). Every execution method calls
`preflight`, which returns, in order: `ErrCollectionRequired` for a nil
Query/collection, the pending builder error, and an error if GroupBy/Having/
aggregation-alias state is pending on a non-Aggregate method (`Find`, `Count`,
`Delete`, `Cursor`, ... all refuse pending group state instead of silently
ignoring it). `Aggregate` uses `preflightBase`, which skips the group-state
check.

### Update normalization (query_exec.go)

`normalizeUpdate` wraps plain documents into `{$set: doc}` when they contain
no `$`-prefixed keys. Handled types: `bson.M`, `map[string]interface{}`,
`bson.D`, structs, and (non-nil) pointers to structs. Anything else passes
through unchanged. An empty document (`bson.M{}`) is still wrapped, producing
`{$set: {}}`, which the server rejects.

### Insert expansion (query_exec.go)

`expandInsertDocs` flattens a single slice/array argument into individual
documents so `Insert(ctx, []*User{...})` works without manual conversion.
`bson.D` (a document that is itself a slice) and `[]byte` are deliberately
left untouched; multi-argument calls pass through as-is.

### Group pipeline construction (query_group.go)

`buildGroupPipeline` emits, in order and only when applicable:
`$match` (Where filter, omitted when empty), `$group` (single key: `_id:
"$field"`; multiple keys: `_id: {flatKey: "$field", ...}`; accumulators from
aggSpecs, count = `{$sum: 1}`), `$project` (drops `_id`, surfaces flattened
group keys and aliases as top-level fields), `$match` (Having conditions built
with the same condition-filter logic), `$sort`, `$skip`, `$limit`. OrderBy,
Offset, and Limit therefore apply to the grouped result rows and must
reference result column names.

### Transaction session propagation (engine.go, query_exec.go)

`Transaction` creates a sub-Engine carrying the session. Every query execution
path calls `execCtx`, which routes through `Engine.sessionContext`: if the
engine holds a session and the caller's ctx does not already carry one, the
session is bound with `mongo.NewSessionContext`. Consequently, queries made
through the `tx` sub-Engine join the transaction even when the plain outer
`ctx` is passed instead of `sc`. Prefer `sc` anyway for correct
deadline/cancellation semantics. `NextSequence` and Cursor getMore/Close also
bind the session. Do not retain `tx` after the callback returns; the session
is ended.

### Naming (naming.go)

`toSnake` inserts an underscore before every uppercase letter past position 0,
so acronyms split letter-by-letter (`HTTPServer` -> `h_t_t_p_server`).
`pluralize` applies, in order: consonant+`y` -> `ies`; ends in `s`/`x`/`sh`/
`ch` -> append `es`; otherwise append `s`. Irregular plurals are not handled.

## Typical usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/hangtiancheng/swifty.go/swifty_orm"
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

type Order struct {
    ID     int64   `bson:"_id"`
    City   string  `bson:"city"`
    Status string  `bson:"status"`
    Amount float64 `bson:"amount"`
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    engine, err := swifty_orm.NewEngine(ctx, "mongodb://localhost:27017", "myapp")
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close(ctx)

    // Insert: variadic docs or a single slice argument (auto-expanded).
    users := []*User{
        {ID: 1, Name: "Alice", Email: "alice@example.com", Age: 30, CreatedAt: time.Now()},
        {ID: 2, Name: "Bob", Email: "bob@example.com", Age: 25, CreatedAt: time.Now()},
    }
    res, err := engine.Collection("users").Insert(ctx, users)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("inserted:", res.InsertedCount)

    // Chainable query with predicates, ordering, pagination, projection.
    var adults []User
    err = engine.Model(&User{}).
        Where("age", ">=", 18).
        WhereNotNull("email").
        WhereLike("name", "A%").
        OrderBy("created_at", "desc").
        Limit(10).
        Offset(0).
        Select("name", "email", "-_id").
        Find(ctx, &adults)
    if err != nil {
        log.Fatal(err)
    }

    // Object form and Or-variants.
    var mixed []User
    err = engine.Collection("users").
        Where(bson.M{"age": 30}).
        OrWhereIn("name", []string{"Bob", "Carol"}).
        Find(ctx, &mixed)
    if err != nil {
        log.Fatal(err)
    }

    // Single document; ErrNotFound when nothing matches.
    var u User
    if err := engine.Model(&User{}).Where("_id", int64(1)).First(ctx, &u); err != nil {
        if err == swifty_orm.ErrNotFound {
            fmt.Println("not found")
        } else {
            log.Fatal(err)
        }
    }

    // Update returns matched count; plain docs auto-wrap in $set.
    matched, err := engine.Collection("users").
        Where("_id", int64(1)).
        Update(ctx, bson.M{"name": "Alice Updated"})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("matched:", matched)

    // Upsert, Increment, Decrement.
    ur, _ := engine.Collection("users").Where("_id", int64(3)).
        Upsert(ctx, bson.M{"name": "Carol", "age": 40})
    fmt.Println("upserted:", ur.UpsertedCount, ur.UpsertedID)
    _, _ = engine.Collection("users").Where("_id", int64(1)).Increment(ctx, "age")
    _, _ = engine.Collection("users").Where("_id", int64(1)).Decrement(ctx, "age", 2)

    // Clone to branch variants from a shared base.
    base := engine.Collection("users").Where("age", ">=", 18)
    activeCount, _ := base.Clone().WhereNotNull("email").Count(ctx)
    total, _ := base.Clone().Count(ctx)
    fmt.Println(activeCount, total)

    // Scalar aggregation and pluck.
    avg, _ := engine.Collection("users").Avg(ctx, "age")
    var names []string
    _ = engine.Collection("users").OrderBy("name").Pluck(ctx, "name", &names)
    n, _ := engine.Collection("users").CountDistinct(ctx, "age")
    fmt.Println(avg, names, n)

    // Grouped aggregation: GroupBy + accumulator aliases + Having + Aggregate.
    type cityAgg struct {
        City  string  `bson:"city"`
        N     int64   `bson:"n"`
        Total float64 `bson:"total"`
    }
    var rows []cityAgg
    err = engine.Collection("orders").
        Where("status", "paid").      // applied before grouping
        GroupBy("city").
        CountAs("n").
        SumAs("amount", "total").
        Having("n", ">=", 2).         // references result columns
        OrderBy("total", "desc").     // references result columns
        Limit(10).
        Aggregate(ctx, &rows)
    if err != nil {
        log.Fatal(err)
    }

    // Streaming, callback style: cursor closed automatically.
    err = engine.Collection("orders").
        Where("status", "paid").
        OrderBy("amount", "desc").
        Each(ctx, func(c *swifty_orm.Cursor) error {
            var o Order
            if err := c.Decode(&o); err != nil {
                return err
            }
            fmt.Println(o.City, o.Amount) // returning an error stops iteration
            return nil
        })
    if err != nil {
        log.Fatal(err)
    }

    // Streaming, manual style: remember to Close.
    cursor, err := engine.Collection("orders").OrderBy("_id").Limit(100).Cursor(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer cursor.Close(ctx)
    for cursor.Next(ctx) {
        var o Order
        if err := cursor.Decode(&o); err != nil {
            log.Fatal(err)
        }
        _ = cursor.Current() // raw bson.Raw of the current document
    }
    if err := cursor.Err(); err != nil {
        log.Fatal(err)
    }

    // Transaction (requires replica set); queries via tx auto-join the session.
    err = engine.Transaction(ctx, func(sc context.Context, tx *swifty_orm.Engine) error {
        if _, err := tx.Collection("users").Where("_id", int64(1)).
            Update(sc, bson.M{"$inc": bson.M{"age": -1}}); err != nil {
            return err
        }
        _, err := tx.Collection("users").Where("_id", int64(2)).
            Update(sc, bson.M{"$inc": bson.M{"age": 1}})
        return err // nil commits, non-nil aborts
    })
    if err != nil {
        log.Fatal(err)
    }

    // Auto-increment sequence and index management.
    nextID, _ := engine.NextSequence(ctx, "order_id")
    fmt.Println("next id:", nextID)
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

1. Empty filter on Update or Delete operates on the entire collection. No
   built-in guard exists against unconditional mass mutation; enforce at
   least one condition at the application layer.
2. A `Where` added after an `OrWhere` still joins the main AND chain:
   `Where(a).OrWhere(b).Where(c)` produces `(a AND c) OR b`, which differs
   from SQL operator precedence (`a OR (b AND c)`). Keep OrWhere calls last,
   or restructure the query.
3. Builder errors are deferred: invalid Where/GroupBy/alias input does not
   fail at the call site; the first recorded error is returned by the next
   execution method. Do not ignore execution errors, or a mistyped operator
   will look like an empty result.
4. `Update` returns MatchedCount, not ModifiedCount. An idempotent update
   (setting a field to its current value) still reports 1. Use `Upsert` when
   you need `ModifiedCount` (its result struct carries both).
5. `Update` with an empty document (`bson.M{}`) is wrapped into `{$set: {}}`,
   which the server rejects. Validate non-empty updates before calling.
6. Struct updates are wrapped in `$set` with all marshaled fields, including
   zero values (subject to `omitempty` bson tags). Use `bson.M` for partial
   updates.
7. Pending GroupBy/Having/alias state on any non-Aggregate execution method
   (Find, Count, Delete, Cursor, ...) is an error by design; only `Aggregate`
   consumes group state, and `Select` cannot be combined with GroupBy.
8. In grouped aggregation, `Having` and `OrderBy` must reference result
   columns: flattened group keys (`addr.city` -> `addr_city`) or accumulator
   aliases. Referencing the raw dotted path or an unknown name is a build
   error. Aliases must not be `_id`, start with `$`, contain `.`, repeat, or
   collide with a group key.
9. Sum/Avg/Min/Max always return `float64`. Sum and Avg return 0 both for "no
   matching documents" and for a genuine zero total, which is
   indistinguishable; Min/Max on non-numeric fields (strings, dates) fail at
   decode. Use `Distinct` or a manual pipeline for non-numeric extremes.
10. Query is a mutable struct; chaining mutates in place. To branch variants
    from a shared base, use `Clone()` -- direct reuse aliases the underlying
    slices.
11. `Pluck` requires a non-nil pointer to a slice of the value type. Documents
    missing the field contribute the element's zero value, indistinguishable
    from a stored zero value.
12. LIKE patterns support only `%` and `_` wildcards; there is no escape
    syntax, so a literal `%` or `_` in the data cannot be matched literally
    through WhereLike/WhereILike (all other regex metacharacters are escaped).
13. Transactions require a MongoDB replica set or sharded cluster; standalone
    mongod rejects them. Queries through the `tx` sub-Engine auto-join the
    session even with a plain ctx, but do not retain `tx` after the callback
    returns -- the session is ended.
14. `NextSequence` uses the hard-coded `counters` collection name. It is not
    configurable and conflicts with application collections of the same name.
15. `CollectionName` splits acronyms letter-by-letter (`HTTPServer` ->
    `h_t_t_p_servers`) and does not handle irregular plurals (`Person` ->
    `persons`). There is no naming-override interface; call
    `engine.Collection("explicit_name")` instead of `Model`.
16. Direct-append condition methods (WhereIn, WhereNull, OrWhereIn, ...) do
    not validate field names; an empty field string produces a filter keyed on
    `""`. Only Where/OrWhere/Having/GroupBy/aliases validate their input.
17. `Distinct` returns `[]interface{}`; callers must type-assert each element.

## File map

| File                 | Purpose                                                                                                                                                             |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `swifty_orm.go`      | Package declaration only (anchor file).                                                                                                                                |
| `engine.go`          | `Engine`, `NewEngine`, accessors, `Collection`, `Model`, `Close`, `DropDatabase`, `NextSequence`, `Transaction`, `sessionContext`.                                     |
| `query.go`           | `Query` struct, builder error accumulation (`setErr`), all chainable condition/or-branch/sort/limit/offset/select methods, `Clone`.                                    |
| `query_exec.go`      | `ErrCollectionRequired`, `ErrNotFound`, `InsertResult`, `UpsertResult`, Insert/First/Find/Update/Upsert/Increment/Decrement/Delete/Count/Exists/EnsureIndexes/DropCollection, `preflight`, `execCtx`, `normalizeUpdate`, `expandInsertDocs`. |
| `query_aggregate.go` | `Sum`, `Avg`, `Min`, `Max`, `Distinct`, `CountDistinct`, `Pluck`, private `aggregate` helper.                                                                           |
| `query_group.go`     | `aggSpec`, `GroupBy`, `Having`, `CountAs`/`SumAs`/`AvgAs`/`MinAs`/`MaxAs`, `Aggregate`, `buildGroupPipeline`, `groupKeyName`.                                           |
| `query_stream.go`    | `Cursor` type and methods (`Next`, `Decode`, `Current`, `Err`, `Close`), `Query.Cursor`, `Query.Each`, session binding for getMore.                                    |
| `filter.go`          | `condition`, `opAliases`, `parseWhere`, `parseWhereMap`, `normalizeOp`, `toBetweenPair`, `likeToRegex`, `buildFilter`, `buildConditionFilter`, `buildProjection`.       |
| `naming.go`          | `CollectionName`, `toSnake`, `pluralize`, `isVowel`.                                                                                                                    |
| `log.go`             | `Error`, `Errorf`, `Info`, `Infof`, `InfoLevel`, `ErrorLevel`, `Disabled`, `SetLevel`.                                                                                  |
| `swifty_orm_test.go` | Unit tests (filter/naming/normalization/pipeline shape) and integration tests (timestamp-isolated database per test; auth failures skip; transaction test skips on standalone mongod). |
| `README.md`          | User-facing documentation, including a Knex.js alignment table.                                                                                                         |
| `swifty_orm_cr.md`   | Code-review notes for the refactor (all blocking findings fixed; see filter composition, Pluck, and session-binding sections above).                                    |
| `go.mod`             | Module declaration, dependencies, replace directives for sibling swifty modules.                                                                                        |

## Dependencies

- Go 1.26.0 or newer (per `go.mod`)
- `go.mongodb.org/mongo-driver` v1.17.6 (direct)

Indirect: `github.com/golang/snappy`, `github.com/klauspost/compress`,
`github.com/montanaflynn/stats`, `github.com/xdg-go/pbkdf2`,
`github.com/xdg-go/scram`, `github.com/xdg-go/stringprep`,
`github.com/youmark/pkcs8`, `golang.org/x/crypto`, `golang.org/x/sync`,
`golang.org/x/text`, `github.com/davecgh/go-spew`, `github.com/google/go-cmp`.

`go.mod` contains `replace` directives pointing the sibling modules
(`swifty_cache`, `swifty_http`, `swifty_rpc`) at their local directories.

Tests require a reachable MongoDB (default `mongodb://localhost:27017/`,
override with `MONGO_URI`); they skip instead of failing when the server
requires authentication, and the transaction test skips on deployments
without replica-set support.

## Cross-references to sibling skills

- `swifty-cache`: Distributed cache framework. Combine swifty_orm for
  persistence with swifty_cache for read-through caching of query results;
  the ORM has no built-in caching layer.
- `swifty-http`: HTTP server framework. Initialize the Engine at server
  startup, inject it into handlers, and pass the request context into query
  methods so HTTP timeouts propagate to MongoDB operations.
- `swifty-rpc`: TCP RPC framework. The Engine provides the persistence layer
  behind RPC method implementations; construct it once per process and share
  it across services.
