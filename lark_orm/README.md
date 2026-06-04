# lark_orm

一个用 Go 编写、专为 MongoDB 设计的轻量级 ORM 框架，API 风格对齐 Node.js 生态中的 Knex 查询构建器，让习惯了链式调用的开发者也能在 Go 中获得熟悉、流畅的查询体验。

底层基于 MongoDB 官方驱动 `go.mongodb.org/mongo-driver`，对外仅暴露两个核心抽象：`Engine`（连接与会话）和 `Query`（链式查询构建器）。整个模块零子包、扁平易读，适合作为业务代码的直接依赖，也方便二次封装。

模块路径：`github.com/hangtiancheng/lark-go/lark_orm`

## 一、特性一览

- 链式 Query 构建器，支持 Where / OrWhere / WhereIn / WhereBetween / WhereNull 等常见谓词
- 自动把不含 MongoDB 操作符的 `bson.M` 包装成 `$set`，写更新更省心
- 内置聚合方法：Count / Sum / Avg / Min / Max / Distinct / Pluck
- 反射推导集合名：传入结构体即可，自动 snake_case 加复数
- 事务封装：基于 MongoDB session，自动提交与回滚
- 自增序列：经典的 `counters` 集合模式，一行调用拿到全局递增 ID
- 索引管理：EnsureIndexes 一次性创建多索引
- 带颜色与级别控制的日志器，可独立开关

## 二、安装

```bash
go get github.com/hangtiancheng/lark-go/lark_orm
```

依赖 Go 1.26 及以上版本。运行单元 / 集成测试需要一台可用的 MongoDB 实例。

## 三、快速开始

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hangtiancheng/lark-go/lark_orm"
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

    engine, err := lark_orm.NewEngine(ctx, "mongodb://localhost:27017", "demo")
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close(ctx)

    // 写入
    _, err = engine.Model(&User{}).Insert(ctx,
        &User{ID: 1, Name: "Alice", Email: "alice@example.com", Age: 28, CreatedAt: time.Now()},
        &User{ID: 2, Name: "Bob",   Email: "bob@example.com",   Age: 19, CreatedAt: time.Now()},
    )
    if err != nil {
        log.Fatal(err)
    }

    // 链式查询
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

    // 更新（不含 $ 操作符的 bson.M 会被自动包成 $set）
    modified, _ := engine.Model(&User{}).
        Where("name", "Alice").
        Update(ctx, bson.M{"age": 29})
    log.Printf("modified=%d", modified)
}
```

## 四、核心概念

### 4.1 Engine

`Engine` 负责管理 MongoDB 连接与数据库句柄，是所有查询的起点。它对 nil 接收者做了防御性判空，调用 getter 时即便 engine 为 nil 也只返回零值，不会 panic。

```go
engine, err := lark_orm.NewEngine(ctx, uri, dbName)

engine.Client()        // *mongo.Client
engine.Database()      // *mongo.Database
engine.DatabaseName()  // string

engine.Close(ctx)        // 断开连接
engine.DropDatabase(ctx) // 删库（慎用）
```

构造时会自动 `Ping` 一次，失败则立即断连并返回错误，避免悬挂的句柄。

### 4.2 Query

`Query` 由 `engine.Collection(name)` 或 `engine.Model(value)` 产生。它持有当前的条件、排序、分页、投影状态，所有链式方法都返回自身，直到调用执行类方法（Find / Insert / Update 等）才真正与 MongoDB 通信。

```go
q := engine.Collection("users")              // 显式集合名
q := engine.Model(&User{})                   // 反射推导，集合名为 users
q := engine.Model([]*User{})                 // 切片也行，结果一致
```

### 4.3 集合命名约定

`engine.Model(value)` 通过反射推导集合名，规则如下：

1. 剥离指针 / 切片，拿到底层结构体
2. 把 CamelCase 转成 snake_case
3. 应用基础英文复数规则

| 结构体        | 集合名           |
| ------------- | ---------------- |
| `User`        | `users`          |
| `ChatHistory` | `chat_histories` |
| `Address`     | `addresses`      |
| `Category`    | `categories`     |
| `Day`         | `days`           |

复数规则覆盖常见情况：辅音 + y 变 ies、s / x / sh / ch 加 es，其余加 s。如果你的命名涉及不规则复数（如 person / people）或缩写（如 HTTPServer），建议显式调用 `engine.Collection("自定义名")`。

## 五、查询构建器

### 5.1 条件方法

```go
q.Where("name", "Alice")                  // name = 'Alice'
q.Where("age", ">=", 18)                  // age >= 18
q.WhereIn("status", []string{"a", "b"})   // status in ('a','b')
q.WhereNotIn("role", []string{"guest"})
q.WhereNull("deleted_at")                 // deleted_at IS NULL
q.WhereNotNull("email")
q.WhereBetween("age", 18, 30)             // age BETWEEN 18 AND 30
q.OrWhere("vip", true)                    // 与主链构成 $or
```

支持的比较操作符（`Where(field, op, value)` 的第二个参数）：

```
=  !=  <>  >  >=  <  <=  $in  $nin
```

操作符无法识别时，会原样下发给 MongoDB，因此你也可以直接传 `$regex`、`$exists` 等原生操作符。

同字段多操作符自动合并：

```go
q.Where("age", ">", 18).Where("age", "<", 30)
// 等价于 { age: { $gt: 18, $lt: 30 } }
```

### 5.2 排序 / 分页 / 投影

```go
q.OrderBy("created_at")           // 默认升序
q.OrderBy("age", "desc")          // 降序，可选 "asc"/"desc"
q.Limit(20)
q.Offset(40)
q.Select("name", "email")         // 投影，等价于 inclusive projection
```

### 5.3 执行方法

```go
// 写入：支持一条或多条
res, err := q.Insert(ctx, doc1, doc2, doc3)
// res.InsertedIDs / res.InsertedCount

// 查询单条
var u User
err := q.First(ctx, &u)            // 无匹配返回 mongo.ErrNoDocuments

// 查询多条
var us []User
err := q.Find(ctx, &us)

// 更新（不含 $ 操作符的 bson.M 会被自动包成 $set）
modified, err := q.Update(ctx, bson.M{"age": 29})
modified, err := q.Update(ctx, bson.M{"$inc": bson.M{"login_count": 1}})

// 删除
deleted, err := q.Delete(ctx)

// 计数 / 存在性
n, err := q.Count(ctx)
ok, err := q.Exists(ctx)

// 索引
names, err := q.EnsureIndexes(ctx, []mongo.IndexModel{...})

// 删除整张集合
err := q.DropCollection(ctx)
```

### 5.4 聚合方法

聚合方法自动复用当前的条件链，底层走 MongoDB 聚合管道 `[$match, $group]`：

```go
sum,  err := engine.Collection("orders").Where("status", "paid").Sum(ctx, "amount")
avg,  err := engine.Collection("orders").Avg(ctx, "amount")
mn,   err := engine.Collection("orders").Min(ctx, "amount")
mx,   err := engine.Collection("orders").Max(ctx, "amount")

vals, err := engine.Collection("users").Distinct(ctx, "city")  // []interface{}

var names []string
err = engine.Collection("users").Where("active", true).Pluck(ctx, "name", &names)
```

Sum / Avg / Min / Max 返回 `float64`，适合数值字段。对非数值字段使用时，建议改用 `Distinct` 或自行通过聚合管道实现。

## 六、事务

事务基于 MongoDB session 与 `WithTransaction` 实现，回调返回 nil 自动提交，返回 error 自动回滚：

```go
err := engine.Transaction(ctx, func(sc context.Context, tx *lark_orm.Engine) error {
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

调用要点：

- 回调内部务必使用入参 `sc` 作为 ctx，否则 driver 无法识别会话上下文，事务不会生效
- 事务要求 MongoDB 部署为副本集或分片集群，单机模式会失败

## 七、自增序列

经典的 counters 集合模式，调用一次得到一个全局递增的 int64：

```go
id, err := engine.NextSequence(ctx, "order_id")  // 第一次返回 1，之后依次 2、3、...
```

底层使用 `FindOneAndUpdate` + `$inc` + upsert，原子性由 MongoDB 保证。每个序列名对应 `counters` 集合中一份独立文档。

## 八、日志

模块提供带颜色前缀与级别控制的日志器：

```go
lark_orm.Info("connecting to mongo")
lark_orm.Infof("matched %d users", n)
lark_orm.Error("oh no")
lark_orm.Errorf("query failed: %v", err)

lark_orm.SetLevel(lark_orm.InfoLevel)   // 全开（最详尽）
lark_orm.SetLevel(lark_orm.ErrorLevel)  // 只保留 Error
lark_orm.SetLevel(lark_orm.Disabled)    // 全关
```

数值越大日志越少：`InfoLevel = 0 < ErrorLevel = 1 < Disabled = 2`。

## 九、错误处理

- 查询执行前 collection 缺失：返回 `lark_orm.ErrCollectionRequired`
- `NewEngine` 校验失败：返回 `mongo uri is required` / `mongo database is required`
- 业务错误：直接透传 driver 原始错误，可用 `errors.Is(err, mongo.ErrNoDocuments)` 等进行判断

建议调用方对常用错误（无匹配文档、重复键、上下文超时）做集中包装。

## 十、文件结构

| 文件                 | 职责                                                                                          |
| -------------------- | --------------------------------------------------------------------------------------------- |
| `engine.go`          | Engine 类型、连接、Collection、Model、Transaction、NextSequence                               |
| `query.go`           | Query 类型与全部链式条件 / 排序 / 分页 / 投影方法                                             |
| `query_exec.go`      | 执行类方法：Insert、First、Find、Update、Delete、Count、Exists、EnsureIndexes、DropCollection |
| `query_aggregate.go` | 聚合方法：Sum、Avg、Min、Max、Distinct、Pluck                                                 |
| `filter.go`          | 条件链到 `bson.M` 过滤器的翻译                                                                |
| `naming.go`          | 结构体名到集合名的转换（snake_case + 复数化）                                                 |
| `log.go`             | 带颜色与级别控制的日志器                                                                      |
| `lark_orm.go`        | 包声明                                                                                        |
| `lark_orm_test.go`   | 单元与集成测试                                                                                |

## 十一、使用注意事项

1. Update / Delete 不带任何 Where 条件时会作用于整张集合，请在业务层显式保证至少有一个条件
2. `Query` 是有状态的可变结构。如果想从一个 base query 分叉出多个变体，建议每次都从 `engine.Collection(...)` 重新开始，避免共享底层切片导致的非预期合并
3. `Pluck` 会修改 Query 的投影字段，调用后请不要再复用同一个 Query 做其他查询
4. `Select` 重复调用会以最后一次为准，仅支持 inclusive 投影
5. 聚合方法仅适合数值字段；字段不存在或非数值时返回 0，无错误
6. 事务回调内必须使用 `sc` 作为 ctx，事务才会真正生效

## 十二、运行测试

```bash
# 默认连接 mongodb://localhost:27017
cd lark_orm
go test ./...

# 自定义 MongoDB 地址
MONGO_URI=mongodb://user:pass@host:27017 go test ./...
```

测试套件会为每个用例创建一个时间戳隔离的临时数据库，跑完自动 Drop。如果目标 MongoDB 开启了鉴权但未提供凭证，集成测试会自动 Skip 而非失败，对 CI 环境友好。

## 十三、与 Knex 的对齐情况

已对齐：

- `where / andWhere / orWhere`
- `whereIn / whereNotIn / whereNull / whereNotNull / whereBetween`
- `orderBy / limit / offset / select`
- `insert / update / del / first / find / count / pluck / distinct`
- `min / max / sum / avg`
- `transaction`

暂未支持（计划中）：

- 回调式分组 `where(qb => { ... })`
- `groupBy / having`
- `upsert` 显式语义
- `whereRaw` / `whereExists` / `whereLike`
- 迭代器 / 流式游标
- 基于泛型的 typed query

## 十四、许可协议

随主仓库统一。详见仓库根目录的 `LICENSE`。
