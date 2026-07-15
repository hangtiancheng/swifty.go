# Swifty Agent 多阶段构建
# 构建: docker build -t swifty-agent .
# 运行: docker run -p 6872:6872 -v $(pwd)/config.json:/app/config.json swifty-agent

# ---- 构建阶段 ----
FROM golang:1.25-alpine AS builder

# 安装 git(go mod 需要)和 ca-certificates
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# 先拷贝 go.mod/go.sum 利用缓存
COPY go.mod go.sum ./
RUN go mod download

# 拷贝源码
COPY . .

# 静态编译(go work 在容器内需单独处理;此处假定 swifty_agent 模块可独立构建)
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/swifty-agent .

# ---- 运行阶段 ----
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# 拷贝二进制
COPY --from=builder /out/swifty-agent /app/swifty-agent

# 默认配置(可被 volume 覆盖)
COPY config.json /app/config.json

EXPOSE 6872

ENTRYPOINT ["/app/swifty-agent"]
