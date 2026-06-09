# KamaChat -> lark_chat 迁移方案

## 一、现状分析

### KamaChat 技术栈

| 层        | 当前技术                   |
| --------- | -------------------------- |
| Web 框架  | Gin (gin-gonic/gin)        |
| 数据库    | MySQL (gorm.io/gorm)       |
| 缓存      | Redis (go-redis/redis)     |
| WebSocket | gorilla/websocket          |
| 消息队列  | Kafka (segmentio/kafka-go) |
| 配置      | TOML (BurntSushi/toml)     |
| 日志      | zap + lumberjack           |
| SMS       | 阿里云短信 SDK             |

### KamaChat 业务模块

1. 用户管理 - 注册、登录、用户信息CRUD、管理员操作
2. 群组管理 - 创建群组、加群、退群、解散群、群信息管理
3. 联系人管理 - 申请好友、通过/拒绝/拉黑、联系人列表
4. 会话管理 - 打开会话、会话列表(用户/群组)、删除会话
5. 消息管理 - 消息列表、文件上传、头像上传
6. WebSocket 实时通信 - 文本消息、文件消息、音视频通话信令
7. 聊天室 - 在线用户列表

### 目标技术栈

| 层        | 目标技术                            |
| --------- | ----------------------------------- |
| Web 框架  | lark_http (Koa风格, middleware链式) |
| 数据库    | MongoDB (lark_orm, mongo-driver)    |
| 缓存      | lark_cache (分布式缓存, LRU, gRPC)  |
| WebSocket | lark_http 内置 ctx.Upgrade()        |
| 消息队列  | Go channel                          |
| 配置      | JSON 配置                           |
| 日志      | Go log 标准库                       |

---

## 二、可行性评估

迁移完全可行，无阻塞项。

### 需要适配的差异

| 差异项        | 解决方案                                                      |
| ------------- | ------------------------------------------------------------- |
| GORM 软删除   | 封装 SoftDelete/ActiveQuery 公共方法，统一 deleted_at 字段    |
| 自增 ID       | 使用 lark_orm.NextSequence                                    |
| 表关系        | 应用层组装                                                    |
| Redis KV 语义 | read-through 场景用 Getter 回源; 纯写入场景(验证码)用 Set/TTL |
| CORS          | 编写 CORS 中间件                                              |

---

## 三、迁移方案

### 目录结构

```
lark_chat/
  cmd/
    server/
      main.go
  configs/
    config.json
  internal/
    config/
      config.go
    middleware/
      cors.go
    model/
      user_info.go
      group_info.go
      user_contact.go
      contact_apply.go
      session.go
      message.go
    dao/
      mongo.go
      cache.go
      soft_delete.go
    service/
      user_service.go
      group_service.go
      contact_service.go
      session_service.go
      message_service.go
      chat_server.go
      chat_client.go
    handler/
      user_handler.go
      group_handler.go
      contact_handler.go
      session_handler.go
      message_handler.go
      ws_handler.go
      response.go
    router/
      router.go
  pkg/
    constants/
      constants.go
    enum/
    util/
      random/
        random.go
  go.mod
```

### 阶段划分

#### 阶段一：基础设施搭建

1. 初始化 go.mod，引入 lark_http、lark_orm、lark_cache
2. JSON 配置加载 (MongoDB URI, 服务端口, 静态资源路径)
3. 初始化 lark_orm Engine
4. 初始化 lark_cache Groups
5. CORS 中间件
6. 统一响应函数
7. 软删除公共方法

#### 阶段二：数据模型迁移

- `gorm:"..."` -> `bson:"..."` + `json:"..."`
- gorm.DeletedAt -> `*time.Time`
- 自增 id -> ObjectID + 业务 uuid 保留

#### 阶段三：数据访问层 + 缓存层

- GORM 调用全部替换为 lark_orm
- read-through 缓存: user_info, session_list, group_session_list, message_list
- 纯 Set 缓存: auth_code (如需要)
- 封装 ActiveQuery (自动 WhereNull deleted_at)
- 封装 SoftDelete (Update deleted_at = now)

#### 阶段四：Handler + 路由

- Gin handler -> lark_http Middleware 签名
- 路由注册

#### 阶段五：WebSocket + ChatServer

- gorilla/websocket -> lark_http ctx.Upgrade()
- 删除 Kafka 相关代码，只保留 channel 模式
- ChatServer 逻辑基本不变

---

## 四、结论

迁移可行，预估工作量 6 天。
