<p align="center">
  <b>xTimer: a distributed timer service implemented in Go</b>
</p>

## Introduction

A distributed timer service built in Go on top of MySQL and Redis. It exposes
timer CRUD APIs, schedules tasks with cron expressions and invokes downstream
services over HTTP when a timer fires.

## Features

- CRUD APIs for timer definitions
- Execution rules defined by cron expressions
- HTTP callbacks to downstream services

## Architecture

A single binary hosts five cooperating process components:

| Component | Responsibility                                                                              |
| --------- | ------------------------------------------------------------------------------------------- |
| webserver | HTTP API for timer CRUD, task queries, `/metrics` endpoint                                  |
| migrator  | Periodically materializes upcoming timer executions into the task table and the Redis zsets |
| scheduler | Splits every minute into time buckets and acquires a distributed lock per bucket            |
| trigger   | Polls the Redis zsets and dispatches due tasks to executors                                 |
| monitor   | Reports enabled-timer and unexecuted-task metrics                                           |

## Getting started

1. Provide a MySQL and a Redis instance.
2. Run the table creation statements in `internal/model/sql`.
3. Fill in the MySQL DSN and the Redis address/password in `conf.json`
   (every other setting has a sensible default, see `internal/config`).
4. Build and run:

```bash
go build ./cmd/xtimer
./xtimer
```

or simply:

```bash
./start.sh
```

The webserver listens on `:8092` by default and pprof is served on `:9999`.

## Configuration

Configuration is read from `conf.json` in the working directory:

```json
{
  "mysql": { "dsn": "user:password@tcp(127.0.0.1:3306)/xtimer" },
  "redis": { "address": "127.0.0.1:6379", "password": "" }
}
```

Optional sections (`migrator`, `scheduler`, `trigger`, `webServer`, tune worker
counts, lock expiries and bucket sizes) fall back to the defaults defined in
`internal/config/config.go`.

## Dependencies

| Dependency                                                | Purpose                                         |
| --------------------------------------------------------- | ----------------------------------------------- |
| [gofiber/fiber/v3](https://github.com/gofiber/fiber/v3)   | HTTP framework for the webserver                |
| [redis/go-redis/v9](https://github.com/redis/go-redis/v9) | Redis client, Lua scripts and distributed locks |
| [gorm.io/gorm](https://gorm.io) + gorm.io/driver/mysql    | MySQL ORM                                       |
| go.uber.org/zap                                           | Structured logging (stdout only)                |
| go.uber.org/dig                                           | Dependency injection container                  |
| prometheus/client_golang                                  | Metrics                                         |

Timer scheduling uses a self-contained cron parser (`internal/cron`), the
worker pools are built on plain goroutines plus a semaphore (`internal/pool`),
and the bloom filter hashes use an internal murmur3/SHA1 implementation
(`internal/hash`).

## Layout

```
cmd/xtimer/          entrypoint (starts all five components)
internal/
  app/               process wiring: provider.go + per-component apps
  config/            conf.json loading and typed config providers
  consts/            shared constants
  model/             persistence objects (po), API view objects (vo), SQL schema
  dao/               MySQL DAOs and Redis task cache
  service/           business logic per component
  utils/             helpers (time formatting, key builders, goroutine ids)
  bloom/             Redis-backed bloom filter
  concurrency/       safe channel helper
  cron/              cron expression parser
  hash/              murmur3 and SHA1 hash helpers
  log/               zap logger (stdout)
  mysql/             gorm client factory
  pool/              bounded goroutine pool
  prometheus/        metrics reporter
  redis/             Redis client wrapper and distributed lock
  xhttp/             JSON HTTP client
```
