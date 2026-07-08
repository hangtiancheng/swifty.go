# Swifty

An AI programming assistant running in the terminal, written in Go.

## Prerequisites

- **Go** 1.25+
- **LLM API key** — set via environment variable or config file:
  - Anthropic: `ANTHROPIC_API_KEY`
  - OpenAI / OpenAI-compatible: `OPENAI_API_KEY`

## Configuration

Swifty loads config from the following paths (later ones override earlier ones):

1. `~/.swifty/config.yaml` — user global
2. `.swifty/config.yaml` — project-level
3. `.swifty/config.local.yaml` — project-local (gitignored, for secrets)

### Config example

```yaml
# .swifty/config.yaml
providers:
  - name: claude
    protocol: anthropic # anthropic | openai | openai-compat
    base_url: https://api.anthropic.com
    model: claude-sonnet-4-5
    api_key: "" # leave empty to read from ANTHROPIC_API_KEY
    thinking: false # enable extended thinking
    context_window: 200000 # optional override
    max_output_tokens: 8192 # optional override

  - name: openai
    protocol: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o
    # api_key read from OPENAI_API_KEY

permission_mode: default # default | yolo

mcp_servers:
  - name: filesystem
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    transport: stdio # stdio | sse | http

hooks: [] # see internal/hooks
```

## Build

### Local build

```bash
go build -o swifty ./cmd/swifty
```

### Cross-platform compilation

Set `GOOS` and `GOARCH` to target a specific OS/architecture:

```bash
# Linux
GOOS=linux   GOARCH=amd64 go build -o bin/swifty-linux-amd64  ./cmd/swifty
GOOS=linux   GOARCH=arm64 go build -o bin/swifty-linux-arm64  ./cmd/swifty

# macOS
GOOS=darwin  GOARCH=amd64 go build -o bin/swifty-darwin-amd64 ./cmd/swifty
GOOS=darwin  GOARCH=arm64 go build -o bin/swifty-darwin-arm64 ./cmd/swifty

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/swifty-windows-amd64.exe ./cmd/swifty
GOOS=windows GOARCH=arm64 go build -o bin/swifty-windows-arm64.exe ./cmd/swifty
```

Build all platforms in one shot via the JS script:

```bash
# from repo root — builds all 6 targets into swifty_cli/bin/
pnpm build:cross
# or directly
node scripts/build_swifty.js
```

Filter by OS or architecture:

```bash
node scripts/build_swifty.js --os=darwin        # only macOS
node scripts/build_swifty.js --arch=arm64       # only arm64
node scripts/build_swifty.js --os=linux --arch=arm64
```

Add `--trimpath` for reproducible, stripped binaries, or `--dry-run` to preview:

```bash
node scripts/build_swifty.js --trimpath
node scripts/build_swifty.js --dry-run
```

### Reproducible build flags

```bash
go build -trimpath -ldflags="-s -w" -o swifty ./cmd/swifty
```

- `-trimpath` strips local filesystem paths from the binary
- `-s -w` strips debug info and DWARF symbol table (smaller binary)

## Development

### Option A: Live reload with [air](https://github.com/air-verse/air)

Install air:

```bash
go install github.com/air-verse/air@latest
```

Run from the `swifty_cli/` directory:

```bash
air
```

Air watches `.go`, `.tpl`, `.tmpl`, `.html` files and automatically rebuilds on changes. Config is in [.air.toml](.air.toml) — the binary is written to `./tmp/swifty`.

### Option B: Without air

Run directly (compile + execute in one step):

```bash
go run ./cmd/swifty
```

## Running

Swifty supports three run modes.

### TUI mode (default)

Interactive terminal UI:

```bash
./swifty
```

### Remote mode

Start an HTTP + WebSocket server; a browser accesses the Web UI:

```bash
./swifty --remote                # listens on :18888
./swifty --remote :8080          # custom port
./swifty --remote 0.0.0.0:8080   # bind to all interfaces
```

### Teammate mode

Run as a worker in a multi-agent team (normally spawned automatically by the lead process, not invoked manually):

```bash
./swifty --teammate --team-name <team> --agent-name <member>
```

## Test

### Run all tests

```bash
go test ./...
```

### Run a specific package

```bash
go test ./internal/skills/...
go test ./internal/tools/...
go test ./internal/agents/...
```

### Verbose with race detector

```bash
go test -v -race ./...
```

### Coverage report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html   # open in browser
go tool cover -func=coverage.out                     # text summary
```

### Run a single test

```bash
go test -run TestLoadCatalogBuiltinsPresent ./internal/skills/
```

### Benchmarks

```bash
go test -bench=. ./internal/tools/
```

## Project structure

```
swifty_cli/
├── cmd/swifty/          # entry point (main.go, teammate.go)
├── internal/
│   ├── agent/           # core agent loop
│   ├── agents/          # agent presets
│   ├── commands/        # slash commands
│   ├── compact/         # context compaction
│   ├── config/          # YAML config loading
│   ├── conversation/    # message history
│   ├── hooks/           # lifecycle hooks
│   ├── llm/             # LLM client abstraction
│   ├── mcp/             # MCP server integration
│   ├── memory/          # memory system
│   ├── permissions/     # tool permission gating
│   ├── plan_file/       # plan tracking
│   ├── prompt/          # system prompt assembly
│   ├── remote/          # --remote HTTP/WS server
│   ├── session/         # session persistence
│   ├── skills/          # skill system + builtins
│   ├── teams/           # multi-agent orchestration
│   ├── todo/            # todo list tracking
│   ├── tools/           # built-in tools (Bash, ReadFile, etc.)
│   ├── tui/             # Bubble Tea terminal UI
│   └── worktree/        # git worktree management
├── .air.toml            # air live-reload config
├── go.mod
└── go.sum
```
