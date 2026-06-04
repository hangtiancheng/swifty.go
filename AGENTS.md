- lark_cache 是 Go 语言编写的分布式缓存
  - lark_cache 的 API 风格期望对齐 groupcache
- lark_http 是 Go 语言编写的 http 框架
  - lark_http 的 API 风格期望对齐 Node.js 的 Koa 框架
- lark_orm 是 Go 语言编写的、只服务于 MongoDB 的 orm 框架
  - lark_orm 的 API 风格期望对齐 Node.js 的 Knex 框架
- lark_rpc 是 Go 语言编写的 rpc 框架
  - lark_rpc 的 API 风格期望对齐 [grpc](https://github.com/grpc/grpc-go)
- lark_demo 是使用 lark_cache, lark_http, lark_orm, lark_rpc 构建的后端服务, 你需要理解这个项目的架构
  - lark_demo 期望与 ai-agent/server 的功能对齐, 具体的对齐方式
    - ai-agent/server 的 redis --> 使用 lark_cache
    - ai-agent/server 的 mysql + knex --> 使用 mongodb + lark_orm
    - ai-agent/server 非流式传输 --> 使用 lark_rpc 非流式转发、lark_http 非流式
    - ai-agent/server 流式传输 --> 使用 lark_rpc SSE 转发、lark_http SSE
    - ai-agent/serve 的 RAG --> lark_demo 使用 langchaingo 支持 RAG
    - ai-agent/serve de1 embedding -> lark_demo 使用 langchaingo 支持 embedding
    - 其他与 Agent、Ollama 交互的地方, lark_demo 使用 langchaingo 优化

对齐顺序

1. 对齐 lark_http
2. 对齐 lark_orm
3. review 一遍 lark_cache
4. review 一遍 lark_rpc
5. 对齐 lark_demo
