# my-raft

`my-raft` is a hands-on Raft consensus implementation written in pure Go
(standard library only, zero third-party dependencies), together with a small
Raft-backed key-value store application that runs on top of it.

## Module

```
github.com/hangtiancheng/swifty.go/apps/my-raft
```

## Layout

```
cmd/my-raft/        # entry point: wires the proxy, KV store, and HTTP API together
internal/raft/      # the Raft consensus algorithm (elections, log replication, Ready/Advance loop)
internal/proxy/     # drives a Raft node: feeds proposals in, publishes committed entries out
internal/kvstore/   # in-memory key-value store that applies committed entries
internal/httpapi/   # HTTP API: PUT /<key> to write, POST /<nodeID> to add a node
```

## How it works

- `internal/raft` implements the Raft algorithm itself: the follower,
  candidate, pre-candidate, and leader roles, leader election (with an
  optional pre-vote phase), log replication, and a `Ready`/`Advance`
  integration loop for storage and networking.
- `internal/proxy` starts a Raft node, ticks it on a timer, and bridges the
  application channels (`proposeC`, `confChangeC`) into Raft proposals.
- `internal/kvstore` applies committed entries to an in-memory map.
- `internal/httpapi` exposes the store over HTTP on port 8091.

## Run

```bash
go run ./cmd/my-raft
```

Write a key/value pair (the key is the request URI, the value is the request
body):

```bash
curl -X PUT http://localhost:8091/foo -d bar
```

Add a node to the cluster (node ID in the path, context in the body):

```bash
curl -X POST http://localhost:8091/2 -d ctx
```

## Build and test

```bash
go build ./...
go vet ./...
go test ./...
```
