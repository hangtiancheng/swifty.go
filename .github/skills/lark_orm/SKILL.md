---
name: lark-orm
description: >
  MongoDB ORM framework (lark_orm module). Use this skill when working on MongoDB
  queries, the chainable Query builder, aggregation operations, collection naming,
  transactions, auto-incrementing sequences, or any code that imports
  github.com/hangtiancheng/lark-go/lark_orm. Also use it when the user asks about Knex-style
  query chaining, BSON filter construction, or lark_orm's Engine lifecycle.
---

# lark_orm

A Knex-inspired, chainable query builder ORM for MongoDB in Go. Provides a fluent
API for CRUD operations, aggregation, transactions, and auto-incrementing sequences
on top of the official `mongo-driver`.

Module path: `github.com/hangtiancheng/lark-go/lark_orm`

Source root: `lark_orm/`

## Architecture overview

```
Engine
  |-- *mongo.Client
  |-- *mongo.Database
  |
  |-- Collection(name) -> *Query
  |-- Model(value)     -> *Query (auto-derives collection name)
  |-- Transaction(ctx, fn)
  |-- NextSequence(ctx, name)

Query (chainable builder)
  |-- Where / WhereIn / WhereNotIn / WhereNull / WhereNotNull / WhereBetween / OrWhere
  |-- OrderBy / Limit / Offset / Select
  |-- Insert / First / Find / Update / Delete / Count / Exists
  |-- Sum / Avg / Min / Max / Distinct / Pluck
  |-- EnsureIndexes / DropCollection
```

## Core types

### Engine

Wraps a MongoDB client + database connection.

```go
func NewEngine(ctx context.Context, uri string, database string) (*Engine, error)
```

`NewEngine` connects, pings, and returns a ready engine. Returns errors for
empty URI/database or connection failure.

Engine methods:

| Method       | Signature                       | Description                                 |
| ------------ | ------------------------------- | ------------------------------------------- |
| Client       | `() -> *mongo.Client`           | Underlying driver client                    |
| Database     | `() -> *mongo.Database`         | Active database handle                      |
| DatabaseName | `() -> string`                  | Database name                               |
| Collection   | `(name string) -> *Query`       | Start a query on a named collection         |
| Model        | `(value interface{}) -> *Query` | Start a query using struct-derived name     |
| Close        | `(ctx) -> error`                | Disconnect the client                       |
| DropDatabase | `(ctx) -> error`                | Drop the entire database                    |
| NextSequence | `(ctx, name) -> (int64, error)` | Atomic auto-increment counter               |
| Transaction  | `(ctx, fn) -> error`            | Run fn inside a MongoDB session transaction |

### Query

Chainable query builder. Created via `engine.Collection(name)` or `engine.Model(v)`.

Condition methods (return `*Query` for chaining):

| Method       | Signature                                | Description                                      |
| ------------ | ---------------------------------------- | ------------------------------------------------ |
| Where        | `(field, value)` or `(field, op, value)` | Equality or comparison filter                    |
| WhereIn      | `(field, values)`                        | `$in` filter                                     |
| WhereNotIn   | `(field, values)`                        | `$nin` filter                                    |
| WhereNull    | `(field)`                                | Field is nil                                     |
| WhereNotNull | `(field)`                                | Field is not nil                                 |
| WhereBetween | `(field, low, high)`                     | `$gte` + `$lte` range                            |
| OrWhere      | `(field, value)` or `(field, op, value)` | `$or` clause                                     |
| OrderBy      | `(field, direction?)`                    | Sort; direction is `"asc"` (default) or `"desc"` |
| Limit        | `(n int64)`                              | Limit result count                               |
| Offset       | `(n int64)`                              | Skip N results                                   |
| Select       | `(fields ...string)`                     | Projection (include only listed fields)          |

Supported comparison operators in `Where(field, op, value)`:
`=`, `!=`, `<>`, `>`, `>=`, `<`, `<=`, `$in`, `$nin`.

Execution methods:

| Method         | Signature                                        | Description                                                             |
| -------------- | ------------------------------------------------ | ----------------------------------------------------------------------- |
| Insert         | `(ctx, docs...) -> (InsertResult, error)`        | Insert one or many documents                                            |
| First          | `(ctx, out) -> error`                            | Find one document (respects sort, skip, projection)                     |
| Find           | `(ctx, out) -> error`                            | Find all matching documents                                             |
| Update         | `(ctx, update) -> (int64, error)`                | Update matching documents; auto-wraps in `$set` if no `$` operator keys |
| Delete         | `(ctx) -> (int64, error)`                        | Delete matching documents                                               |
| Count          | `(ctx) -> (int64, error)`                        | Count matching documents                                                |
| Exists         | `(ctx) -> (bool, error)`                         | True if any document matches                                            |
| EnsureIndexes  | `(ctx, []mongo.IndexModel) -> ([]string, error)` | Create indexes                                                          |
| DropCollection | `(ctx) -> error`                                 | Drop the underlying collection                                          |

### InsertResult

```go
type InsertResult struct {
    InsertedIDs   []interface{}
    InsertedCount int64
}
```

### Aggregation methods

All aggregation methods respect the current filter chain.

| Method   | Signature                                | Description                       |
| -------- | ---------------------------------------- | --------------------------------- |
| Sum      | `(ctx, field) -> (float64, error)`       | Sum of field values               |
| Avg      | `(ctx, field) -> (float64, error)`       | Average of field values           |
| Min      | `(ctx, field) -> (float64, error)`       | Minimum field value               |
| Max      | `(ctx, field) -> (float64, error)`       | Maximum field value               |
| Distinct | `(ctx, field) -> ([]interface{}, error)` | Distinct field values             |
| Pluck    | `(ctx, field, out) -> error`             | Find with single-field projection |

Aggregation uses a MongoDB aggregation pipeline:
`[{$match: filter}, {$group: {_id: null, result: accumulator}}]`.

## Collection naming

`engine.Model(value)` derives the collection name from the struct type via
`CollectionName(value)`:

1. Unwrap pointers and slices to reach the struct type
2. Convert `CamelCase` to `snake_case`
3. Pluralize with basic English rules (consonant+y -> ies, s/x/sh/ch -> es, else -> s)

Examples:

- `User` -> `users`
- `ChatHistory` -> `chat_histories`
- `Address` -> `addresses`

## Filter construction

The `buildFilter()` method translates the condition chain into a `bson.M`:

- Each `Where` with `"="` sets `field: value` directly
- Other operators map through `opMap` (e.g., `"!="` -> `"$ne"`, `">"` -> `"$gt"`)
- `WhereNull` sets `field: nil`, `WhereNotNull` sets `field: {$ne: nil}`
- `WhereBetween` merges `{$gte: low, $lte: high}` on the same field
- Multiple conditions on the same field merge into a single sub-document
- `OrWhere` wraps all conditions in `{$or: [base, orGroup1, ...]}`

## Transactions

```go
err := engine.Transaction(ctx, func(sc context.Context, tx *Engine) error {
    _, err := tx.Collection("accounts").
        Where("_id", fromID).
        Update(sc, bson.M{"$inc": bson.M{"balance": -amount}})
    if err != nil {
        return err
    }
    _, err = tx.Collection("accounts").
        Where("_id", toID).
        Update(sc, bson.M{"$inc": bson.M{"balance": amount}})
    return err
})
```

The `tx` engine shares the same client and database but carries the session.
The transaction auto-commits on nil return, auto-aborts on error.

## Auto-increment sequences

```go
nextID, err := engine.NextSequence(ctx, "order_id")
```

Uses a `counters` collection with `FindOneAndUpdate` + `$inc` + upsert.
Each sequence name gets its own counter document.

## Typical usage

```go
engine, err := lark_orm.NewEngine(ctx, "mongodb://localhost:27017", "mydb")
defer engine.Close(ctx)

// Insert
result, _ := engine.Collection("users").Insert(ctx,
    bson.M{"name": "Alice", "age": 30},
    bson.M{"name": "Bob", "age": 25},
)

// Query with chaining
var users []User
engine.Model(&User{}).
    Where("age", ">=", 18).
    WhereNotNull("email").
    OrderBy("created_at", "desc").
    Limit(10).
    Select("name", "email").
    Find(ctx, &users)

// Aggregation
avg, _ := engine.Collection("orders").
    Where("status", "completed").
    Avg(ctx, "total")

// Count
count, _ := engine.Model(&User{}).
    Where("active", true).
    Count(ctx)
```

## File map

| File                 | Purpose                                                               |
| -------------------- | --------------------------------------------------------------------- |
| `engine.go`          | Engine type, connection, Collection, Model, Transaction, NextSequence |
| `query.go`           | Query builder with condition/sort/limit/offset/select methods         |
| `query_exec.go`      | Execution methods: Insert, First, Find, Update, Delete, Count, Exists |
| `query_aggregate.go` | Aggregation: Sum, Avg, Min, Max, Distinct, Pluck                      |
| `filter.go`          | BSON filter construction from condition chain                         |
| `naming.go`          | Struct-to-collection-name conversion (snake_case + pluralize)         |
| `lark_orm.go`        | Package declaration                                                   |
| `log/`               | Logging utilities                                                     |

## Dependencies

- `go.mongodb.org/mongo-driver` -- MongoDB driver

## Sentinel errors

- `ErrCollectionRequired` -- query executed without a backing collection
