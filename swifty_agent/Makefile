# Swifty Agent Makefile
# 统一构建/运行/测试入口

GO ?= go
APP := swifty-agent
PKG := ./...

.PHONY: all build run dev test vet tidy fmt clean docker-up docker-down docker-build help

all: build

# 编译二进制到 bin/
build:
	$(GO) build -o bin/$(APP) .

# 直接运行
run:
	$(GO) run .

# air 热重载开发
dev:
	air

# 运行测试
test:
	$(GO) test $(PKG) -race -cover

# go vet
vet:
	$(GO) vet $(PKG)

# go mod tidy
tidy:
	$(GO) mod tidy

# 格式化
fmt:
	$(GO) fmt $(PKG)

# 清理构建产物
clean:
	rm -rf bin/ tmp/

# 启动基础设施(Redis/Prometheus/Grafana)
docker-up:
	docker compose up -d

# 停止基础设施
docker-down:
	docker compose down

# 构建镜像
docker-build:
	docker build -t $(APP) .

help:
	@echo "Swifty Agent Makefile targets:"
	@echo "  make build        编译到 bin/$(APP)"
	@echo "  make run          go run ."
	@echo "  make dev          air 热重载"
	@echo "  make test         运行测试"
	@echo "  make vet          go vet"
	@echo "  make tidy         go mod tidy"
	@echo "  make fmt          go fmt"
	@echo "  make clean        清理 bin/ tmp/"
	@echo "  make docker-up    启动 Redis/Prometheus/Grafana"
	@echo "  make docker-down  停止基础设施"
	@echo "  make docker-build 构建镜像"
