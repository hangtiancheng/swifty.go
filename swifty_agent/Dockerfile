# Swifty Agent multi-stage build
# Build: docker build -t swifty-agent .
# Run:   docker run -p 6872:6872 -v $(pwd)/config.json:/app/config.json swifty-agent

# ---- Build stage ----
FROM golang:1.25-alpine AS builder

# Install git (required by go mod) and ca-certificates
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go.mod/go.sum first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Static compilation (go work requires separate handling inside the container;
# this assumes the swifty_agent module can be built independently)
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/swifty-agent .

# ---- Runtime stage ----
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /out/swifty-agent /app/swifty-agent

# Default configuration (can be overridden via volume mount)
COPY config.json /app/config.json

EXPOSE 6872

ENTRYPOINT ["/app/swifty-agent"]
