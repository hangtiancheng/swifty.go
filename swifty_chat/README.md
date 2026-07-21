# swifty_chat

A chat server built on the swifty.go stack: swifty_http (HTTP + WebSocket),
swifty_orm (MongoDB) and swifty_cache (in-process read-through cache).

## Run

```bash
go run ./cmd            # backend, reads ./config.json
cd fe && pnpm dev       # frontend dev server
```

## Deployment constraints

- **Single instance only.** The message bus is an in-process channel, the
  WebSocket connection table and the cache both live in process memory.
  Messages sent to a user connected to another instance would never be
  delivered. Plan capacity for one instance.
- **No TLS.** The server speaks plain HTTP/WS. For anything beyond an
  internal network, terminate TLS at a gateway (nginx, caddy, ...) in front
  of it. Passwords travel and are stored in plain text.
- **MongoDB transactions** require a replica set. On a standalone mongod the
  server automatically falls back to sequential (non-transactional) writes.
- The `GET /dashboard/ws` endpoint exposes the swifty_cache monitoring
  dashboard without authentication; restrict access to it in production.
