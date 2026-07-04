# Swifty ORM

A lightweight MongoDB ORM for Go, designed around a chainable query builder inspired by Knex.js. It provides a fluent, expressive API for developers accustomed to method chaining while retaining the full power of the official MongoDB driver underneath.

Built on top of `go.mongodb.org/mongo-driver`, the library exposes two core abstractions: `Engine` (connection and session management) and `Query` (chainable query builder). The entire module is flat (no sub-packages), easy to read, and suitable as a direct dependency in application code or as a foundation for higher-level wrappers.

Module path: `github.com/hangtiancheng/swifty.go/swifty_orm`

## Features

- Chainable query builder with Where / OrWhere / WhereIn / WhereNotIn / WhereBetween / WhereNull / WhereNotNull predicates
- Automatic `$set` wrapping: plain `bson.M` updates without MongoDB operators are transparently wrapped in `$set`
- Built-in aggregation methods: Count / Sum / Avg / Min / Max / Distinct / Pluck
- Reflection-based collection naming: pass a struct and the collection name is derived automatically (snake_case + pluralization)
- Transaction support: wraps MongoDB sessions with automatic commit and rollback
- Auto-increment sequences: classic `counters` collection pattern for globally unique sequential IDs
- Index management: create multiple indexes in a single call via EnsureIndexes
- Colored, level-controlled logger with independent toggle

## Installation

```bash
go get github.com/hangtiancheng/swifty.go/swifty_orm
```

Requires Go 1.26 or later. A running MongoDB instance is required for integration tests.

## Quick Start

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hangtiancheng/swifty.go/swifty_orm"
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

    engine, err := swifty_orm.NewEngine(ctx, "mongodb://localhost:27017", "demo")
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close(ctx)

    // Insert documents
    _, err = engine.Model(&User{}).Insert(ctx,
        &User{ID: 1, Name: "Alice", Email: "alice@example.com", Age: 28, CreatedAt: time.Now()},
        &User{ID: 2, Name: "Bob",   Email: "bob@example.com",   Age: 19, CreatedAt: time.Now()},
    )
    if err != nil {
        log.Fatal(err)
    }

    // Chainable query
    var adults []User
    err = engine.Model(&User{}).
        Where("age", ">=", 18).
        WhereNotNull("email").
        OrderBy("created_at", "desc").
        Limit(10).
        Find(ctx, &adults)
    if err != nil {
        log.Fatal(err)
    }

    // Update (plain bson.M without $ operators is automatically wrapped in $set)
    modified, _ := engine.Model(&User{}).
        Where("name", "Alice").
        Update(ctx, bson.M{"age": 29})
    log.Printf("modified=%d", modified)
}
```

## Core Concepts

### Engine

`Engine` manages the MongoDB connection and database handle. It serves as the entry point for all queries. Nil-receiver safety is built in: calling getters on a nil Engine returns zero values without panicking.

```go
engine, err := swifty_orm.NewEngine(ctx, uri, dbName)

engine.Client()        // *mongo.Client
engine.Database()      // *mongo.Database
engine.DatabaseName()  // string

engine.Close(ctx)        // disconnect
engine.DropDatabase(ctx) // drop the entire database
```

During construction, `NewEngine` issues a Ping. If the ping fails, the connection is immediately closed and an error is returned, preventing dangling handles.

### Query

A `Query` is created via `engine.Collection(name)` or `engine.Model(value)`. It accumulates conditions, sort order, pagination, and projection state. All chainable methods return the same `Query` pointer. Actual communication with MongoDB only occurs when a terminal method (Find, Insert, Update, etc.) is invoked.

```go
q := engine.Collection("users")   // explicit collection name
q := engine.Model(&User{})        // derived via reflection -> "users"
q := engine.Model([]*User{})      // slices also work -> "users"
```

### Collection Naming Convention

`engine.Model(value)` derives the collection name through reflection:

1. Strip pointers and slices to reach the underlying struct type
2. Convert CamelCase to snake_case
3. Apply English pluralization rules

| Struct        | Collection Name  |
| ------------- | ---------------- |
| `User`        | `users`          |
| `ChatHistory` | `chat_histories` |
| `Address`     | `addresses`      |
| `Category`    | `categories`     |
| `Day`         | `days`           |

Pluralization rules cover common cases: consonant + y becomes -ies; endings in s / x / sh / ch receive -es; everything else gets -s. For irregular plurals (e.g., person/people) or acronyms (e.g., HTTPServer), use `engine.Collection("custom_name")` directly.

## Query Builder

### Condition Methods

```go
q.Where("name", "Alice")                  // name == "Alice"
q.Where("age", ">=", 18)                  // age >= 18
q.WhereIn("status", []string{"a", "b"})   // status in ["a", "b"]
q.WhereNotIn("role", []string{"guest"})   // status not in ["guest"]
q.WhereNull("deleted_at")                 // deleted_at == null
q.WhereNotNull("email")                   // email != null
q.WhereBetween("age", 18, 30)             // 18 <= age <= 30
q.OrWhere("vip", true)                    // combined with the main chain via $or
```

Supported comparison operators (second argument in `Where(field, op, value)`):

```
=  !=  <>  >  >=  <  <=  $in  $nin
```

Unrecognized operators are passed through to MongoDB as-is, so native operators like `$regex` or `$exists` can be used directly.

Multiple operators on the same field are merged automatically:

```go
q.Where("age", ">", 18).Where("age", "<", 30)
// produces { age: { $gt: 18, $lt: 30 } }
```

### Sorting, Pagination, and Projection

```go
q.OrderBy("created_at")           // ascending (default)
q.OrderBy("age", "desc")          // descending; accepts "asc" or "desc"
q.Limit(20)
q.Offset(40)
q.Select("name", "email")         // inclusive projection
```

### Terminal Methods

```go
// Insert: single or multiple documents
res, err := q.Insert(ctx, doc1, doc2, doc3)
// res.InsertedIDs / res.InsertedCount

// Retrieve a single document
var u User
err := q.First(ctx, &u)            // returns mongo.ErrNoDocuments if nothing matches

// Retrieve multiple documents
var us []User
err := q.Find(ctx, &us)

// Update (plain bson.M without $ operators is auto-wrapped in $set)
modified, err := q.Update(ctx, bson.M{"age": 29})
modified, err := q.Update(ctx, bson.M{"$inc": bson.M{"login_count": 1}})

// Delete
deleted, err := q.Delete(ctx)

// Count and existence check
n, err := q.Count(ctx)
ok, err := q.Exists(ctx)

// Index management
names, err := q.EnsureIndexes(ctx, []mongo.IndexModel{...})

// Drop an entire collection
err := q.DropCollection(ctx)
```

### Aggregation Methods

Aggregation methods automatically respect the current condition chain. Internally they execute a MongoDB aggregation pipeline consisting of `[$match, $group]`:

```go
sum, err := engine.Collection("orders").Where("status", "paid").Sum(ctx, "amount")
avg, err := engine.Collection("orders").Avg(ctx, "amount")
mn,  err := engine.Collection("orders").Min(ctx, "amount")
mx,  err := engine.Collection("orders").Max(ctx, "amount")

vals, err := engine.Collection("users").Distinct(ctx, "city")  // []interface{}

var names []string
err = engine.Collection("users").Where("active", true).Pluck(ctx, "name", &names)
```

Sum / Avg / Min / Max return `float64`. They are designed for numeric fields. For non-numeric fields, use Distinct or build a custom aggregation pipeline.

## Transactions

Transactions are built on MongoDB sessions and `WithTransaction`. Returning nil from the callback triggers an automatic commit; returning an error triggers an automatic rollback:

```go
err := engine.Transaction(ctx, func(sc context.Context, tx *swifty_orm.Engine) error {
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

Important notes:

- Always use the `sc` parameter as the context inside the callback; otherwise the driver cannot associate operations with the session and the transaction will not take effect.
- Transactions require a MongoDB replica set or sharded cluster deployment. They will fail on standalone instances.

## Auto-Increment Sequences

The classic counters collection pattern. Each call returns a globally incrementing `int64`:

```go
id, err := engine.NextSequence(ctx, "order_id")  // returns 1, then 2, 3, ...
```

Under the hood this uses `FindOneAndUpdate` with `$inc` and upsert. Atomicity is guaranteed by MongoDB. Each sequence name corresponds to a separate document in the `counters` collection.

## Logging

The module provides a colored, level-controlled logger:

```go
swifty_orm.Info("connecting to mongo")
swifty_orm.Infof("matched %d users", n)
swifty_orm.Error("connection lost")
swifty_orm.Errorf("query failed: %v", err)

swifty_orm.SetLevel(swifty_orm.InfoLevel)   // all logs enabled (most verbose)
swifty_orm.SetLevel(swifty_orm.ErrorLevel)  // errors only
swifty_orm.SetLevel(swifty_orm.Disabled)    // all logs suppressed
```

Numeric ordering: `InfoLevel (0) < ErrorLevel (1) < Disabled (2)`. Higher values suppress more output.

## Error Handling

- Missing collection before query execution: returns `swifty_orm.ErrCollectionRequired`
- `NewEngine` validation failures: returns `"mongo uri is required"` or `"mongo database is required"`
- All other errors: the original driver error is returned transparently; use `errors.Is(err, mongo.ErrNoDocuments)` or similar for structured handling

It is recommended that callers build centralized error-wrapping for common cases (no matching document, duplicate key, context timeout).

## File Structure

| File                 | Responsibility                                                                                      |
| -------------------- | --------------------------------------------------------------------------------------------------- |
| `engine.go`          | Engine type, connection lifecycle, Collection, Model, Transaction, NextSequence                     |
| `query.go`           | Query type and all chainable condition / sort / pagination / projection methods                     |
| `query_exec.go`      | Terminal methods: Insert, First, Find, Update, Delete, Count, Exists, EnsureIndexes, DropCollection |
| `query_aggregate.go` | Aggregation methods: Sum, Avg, Min, Max, Distinct, Pluck                                            |
| `filter.go`          | Translation of the condition chain into a `bson.M` filter document                                  |
| `naming.go`          | Struct name to collection name conversion (snake_case + pluralization)                              |
| `log.go`             | Colored, level-controlled logger                                                                    |
| `swifty_orm.go`      | Package declaration                                                                                 |
| `swifty_orm_test.go` | Unit and integration tests                                                                          |

## Usage Notes

1. Update and Delete without any Where conditions will affect the entire collection. Ensure at least one condition is present at the application layer.
2. `Query` is a mutable, stateful struct. To branch multiple query variants from a common base, always start fresh from `engine.Collection(...)` to avoid unintended state sharing through underlying slices.
3. `Pluck` mutates the Query's projection fields. Do not reuse the same Query for other operations after calling Pluck.
4. Calling `Select` multiple times replaces the previous projection. Only inclusive projection is supported.
5. Aggregation methods are designed for numeric fields. If the field does not exist or is not numeric, they return 0 with no error.
6. Inside transaction callbacks, the `sc` context parameter must be used for all operations; otherwise the transaction will not take effect.

## Running Tests

```bash
# Connects to mongodb://localhost:27017 by default
cd swifty_orm
go test ./...

# Custom MongoDB URI
MONGO_URI=mongodb://user:pass@host:27017 go test ./...
```

The test suite creates a timestamp-isolated temporary database for each test case and drops it automatically on cleanup. If the target MongoDB instance requires authentication but no credentials are provided, integration tests will be skipped (not failed), making them CI-friendly.

## Knex.js Alignment

Implemented:

- `where / andWhere / orWhere`
- `whereIn / whereNotIn / whereNull / whereNotNull / whereBetween`
- `orderBy / limit / offset / select`
- `insert / update / del / first / find / count / pluck / distinct`
- `min / max / sum / avg`
- `transaction`

Not yet implemented (planned):

- Callback-based grouping `where(qb => { ... })`
- `groupBy / having`
- Explicit `upsert` semantics
- `whereRaw` / `whereExists` / `whereLike`
- Iterator / streaming cursor
- Generics-based typed query

## License

Governed by the main repository license. See the `LICENSE` file in the repository root.
