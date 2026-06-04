# lark_orm 代码评审

本文档对 `lark_orm` 模块进行一次系统性、细粒度的代码评审，目标是：

1. 厘清模块整体架构与各文件职责
2. 逐文件审视实现细节、潜在缺陷与改进建议
3. 给出可执行的优化清单，作为后续迭代的依据

评审范围：`lark_orm/` 目录下的全部 Go 源文件（含测试文件），共计约 1011 行。

## 一、模块定位与整体架构

### 1.1 定位

`lark_orm` 是一个仅服务于 MongoDB 的 ORM 框架，使用 Go 语言实现，构建在官方驱动 `go.mongodb.org/mongo-driver` 之上。其 API 风格刻意对齐 Node.js 生态中的 Knex 查询构建器，提供链式调用的查询表达能力。

### 1.2 整体架构

模块采用扁平包结构（所有类型均位于 `lark_orm` 包），核心抽象只有两个：

- `Engine`：连接生命周期管理者，封装 `*mongo.Client` 与 `*mongo.Database`，是所有查询的入口
- `Query`：链式查询构建器，承载条件、排序、分页、投影状态，并提供执行方法

调用链路为：

```
NewEngine(uri, db)
   │
   ├── Collection(name) ──► *Query ──► Where/OrderBy/Limit/Select ──► Find/Insert/Update/Delete
   │
   ├── Model(value)     ──► *Query（通过反射推导集合名）
   │
   ├── Transaction(fn)   ──► 在 SessionContext 内复用同一 Engine
   │
   └── NextSequence(name) ──► 基于 counters 集合的原子自增
```

### 1.3 文件职责

| 文件                 | 行数 | 职责                                                                                          |
| -------------------- | ---- | --------------------------------------------------------------------------------------------- |
| `lark_orm.go`        | 1    | 包声明占位                                                                                    |
| `engine.go`          | 127  | Engine 类型、连接、Collection、Model、Transaction、NextSequence                               |
| `query.go`           | 76   | Query 类型与全部条件 / 排序 / 分页 / 投影方法                                                 |
| `query_exec.go`      | 152  | 执行类方法：Insert、First、Find、Update、Delete、Count、Exists、EnsureIndexes、DropCollection |
| `query_aggregate.go` | 60   | 聚合类方法：Sum、Avg、Min、Max、Distinct、Pluck                                               |
| `filter.go`          | 93   | 条件链到 `bson.M` 过滤器的翻译，以及投影构建                                                  |
| `naming.go`          | 54   | 结构体名 → snake_case 复数集合名                                                              |
| `log.go`             | 47   | 带颜色与级别控制的简单日志器                                                                  |
| `lark_orm_test.go`   | 401  | 单元 / 集成测试                                                                               |

## 二、分文件详细评审

### 2.1 lark_orm.go

仅包含一行 `package lark_orm`。文件本身没有问题，但作为包入口文件预期承担两类职责：

- 包级文档注释（doc.go 模式）
- 公共哨兵错误集中定义

目前 `ErrCollectionRequired` 定义在 `query_exec.go`，而非这里。建议把所有 `Err*` 哨兵错误集中迁移到 `lark_orm.go`（或新建 `errors.go`），以便调用方阅读和 `errors.Is` 检查时有统一来源。

### 2.2 engine.go

实现质量整体良好，并对 nil 接收者做了防御性判空。逐项展开：

#### NewEngine

```go
func NewEngine(ctx context.Context, uri string, database string) (*Engine, error)
```

正面：

- 对空 uri 与空数据库名做了 `TrimSpace` 校验，给出明确错误
- `Ping` 失败时主动 `Disconnect`，避免句柄泄漏

潜在改进：

- `mongo.Connect` 是同步连接接口，但底层是惰性的；`Ping` 通过后并不能完全代表 URI 中所有节点可达。可以考虑暴露 `*options.ClientOptions` 透传入口，让调用方可以注入读写偏好、连接池大小、超时策略等
- 错误信息未用 `fmt.Errorf("%w", err)` 包裹，调用方无法 `errors.Is/As` 出底层驱动错误。建议改为 `return nil, fmt.Errorf("lark_orm: connect: %w", err)`
- 没有任何选项参数，未来扩展会破坏签名兼容。建议引入 `Option` 函数式选项

#### Client / Database / DatabaseName

三个 getter 都做了 nil 接收者保护，返回零值。这是较好的实践。建议为 `Engine` 加上 `Session() mongo.Session`，让事务内的高级用法可以拿到 session 做手动控制（目前 `session` 字段虽存在但完全未对外暴露）。

#### Collection / Model

```go
func (e *Engine) Collection(name string) *Query {
    var col *mongo.Collection
    if e != nil && e.database != nil && strings.TrimSpace(name) != "" {
        col = e.database.Collection(name)
    }
    return &Query{collection: col, engine: e}
}
```

注意点：

- 当 `name` 为空时返回的 `Query` 持有 nil collection，后续任何执行方法都会返回 `ErrCollectionRequired`，这是合理的失败前置策略
- `Model` 通过 `CollectionName(value)` 推导名称，逻辑清晰

潜在问题：

- 返回的 `*Query` 没有持有 `context.Context`，所有执行方法都必须再次传入 ctx。这与 Knex 的链式风格略有差异，但与 Go 习惯一致，可接受
- 当 `e == nil` 时 `Query.engine` 也会是 nil，但目前 `Query` 内部不依赖 `engine` 字段（除了存储），所以不会立即崩溃；建议要么移除该字段，要么明确其用途（例如供事务侦测使用）

#### NextSequence

```go
counters := e.database.Collection("counters")
opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
update := bson.M{"$inc": bson.M{"value": int64(1)}}
```

实现是 MongoDB 自增序列的经典模式，正确且原子。注意：

- 集合名 `counters` 是硬编码，对接已有系统时可能冲突。建议提供可配置项，例如 `NewEngine` 的 Option 中加入 `WithSequenceCollection("seq")`
- 字段名 `value` 未导出为常量，重构时易出错
- 返回类型为 `int64`，但 `$inc` 的整数会在 BSON 中表现为 `Int32` 或 `Int64`，依赖 driver 自行解码到 `int64`。当前测试覆盖了从 1 到 2 的递增是正确的；建议在文档中明示首次调用即返回 1，便于上游使用

#### Transaction

```go
session, err := e.client.StartSession()
...
_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
    txEngine := &Engine{ client, database, databaseName, session }
    return nil, fn(sc, txEngine)
})
```

实现思路正确：复用同一 client 与 database，但携带 session 字段。值得关注的是：

- `txEngine.Collection(...)` 返回的 `*Query` 并没有将 `mongo.SessionContext` 与底层 `*mongo.Collection` 绑定。MongoDB 驱动是通过 `ctx` 传递 session 上下文的，因此实际执行时只要调用方使用回调入参 `sc` 作为 ctx，事务就会正确生效
- 但是 `Engine.session` 字段并没有被任何代码读取，纯粹是冗余存储。如果未来想做"事务内自动注入 session" 的封装，可以基于该字段做改造；当前阶段建议要么删除，要么写注释说明用途
- 没有提供 `TransactionOptions`（读关注、写关注、读偏好），生产环境可能需要
- 当 MongoDB 部署为单机时事务会失败，建议在文档中提示"需要副本集或分片集群"

### 2.3 query.go

定义了 `Query` 结构体与全部条件构造方法。整体非常简洁：

```go
type Query struct {
    collection *mongo.Collection
    engine     *Engine
    conditions []condition
    orGroups   [][]condition
    sort       bson.D
    limit      int64
    skip       int64
    fields     []string
}
```

#### 链式语义的隐藏问题

`Query` 是可变结构体，每个链式方法直接 `append` 并返回自身。这意味着：

```go
base := engine.Collection("users").Where("active", true)
q1 := base.Where("age", ">", 18)
q2 := base.Where("age", ">", 30)
// q1 与 q2 共享同一 conditions 切片底层数组
```

由于 `append` 的语义，当 cap 足够时两次 append 可能互相覆盖，导致 `q1` 实际包含的条件不可预期。这是一个真实的、容易踩到的坑，对齐 Knex 时尤其敏感（Knex 通过 `clone()` 解决）。建议：

- 要么明确文档说明 `Query` 不可复用、不可分叉，建议每次从 `Collection/Model` 开始
- 要么实现 `Clone()` 方法，并在每个链式方法内部使用值传递的 `conditions`
- 最稳妥：在每个链式方法中做防御性拷贝（性能开销可忽略）

#### Where 的参数解析

```go
func parseWhere(args ...interface{}) condition {
    switch len(args) {
    case 2:
        return condition{field: args[0].(string), op: "=", value: args[1]}
    case 3:
        return condition{field: args[0].(string), op: args[1].(string), value: args[2]}
    default:
        return condition{}
    }
}
```

观察：

- 当 `args` 既不是 2 个也不是 3 个时，静默返回零值 `condition{}`，这会被 `applyCondition` 当作 `op="="` 的空字段条件处理，污染过滤器。建议改为 `panic` 或将 `parseWhere` 改返回 `(condition, error)`，并由调用方记录错误
- `args[0].(string)` 与 `args[1].(string)` 的类型断言失败时会 panic，没有保护。考虑到 Knex 的语义就是字符串字段名，panic 在开发期能快速暴露问题，但建议在错误信息中给出更友好的提示

#### OrWhere 的语义

```go
func (q *Query) OrWhere(args ...interface{}) *Query {
    q.orGroups = append(q.orGroups, []condition{parseWhere(args...)})
    return q
}
```

每次 `OrWhere` 都会新建一个独立 group，与 `filter.go` 中 `$or` 的拼装一致。但缺少 "或组里再加 And" 的能力。Knex 是支持 `.where(function(){ this.where().orWhere() })` 的回调式分组的，本框架尚未提供。后续可考虑：

```go
func (q *Query) OrWhereGroup(build func(g *Query)) *Query
```

#### OrderBy 的方向解析

```go
if len(direction) > 0 && (direction[0] == "desc" || direction[0] == "DESC") {
    dir = -1
}
```

只识别 `"desc" / "DESC"`，混合大小写如 `"Desc"` 不识别。考虑使用 `strings.EqualFold`。

#### Limit / Offset

`limit` 与 `skip` 默认 0。`query_exec.go` 在 0 时不下发选项，这是正确的。但要注意：MongoDB 允许 limit 为负数（表示批量大小并关闭游标），本实现不支持，符合预期。

#### Select 的语义

```go
func (q *Query) Select(fields ...string) *Query {
    q.fields = fields
    return q
}
```

直接覆盖切片而非追加。如果调用方多次 `Select`，最后一次会胜出。文档中需要明示。另外目前只支持 inclusive 投影（设值为 1），不支持 exclusive 投影（设值为 0）。可以扩展 `Omit(fields ...string)` 与之对偶。

### 2.4 query_exec.go

#### Insert

```go
if len(documents) == 1 {
    result, err := q.collection.InsertOne(ctx, documents[0])
    ...
}
result, err := q.collection.InsertMany(ctx, documents)
```

实现合理。注意：

- `documents` 为 `...interface{}`，调用方既能传结构体也能传 `bson.M`，灵活性高
- `InsertedCount` 直接取自 `len(result.InsertedIDs)`，与 driver 的 `InsertedCount` 字段语义一致（驱动本身没有 InsertedCount 字段，只能这么取，正确）
- 单条插入时手动构造 `InsertResult`，多条插入时复用结果。代码统一性可以更好，例如统一走 `InsertMany` 单文档场景，代价是 driver 内部多一次切片操作

#### First / Find

实现规整。`First` 不用 `limit`，因为 `FindOne` 本身只返回一条。两处都在 `q.skip > 0` 时才下发，避免传 0。

潜在问题：

- `Find` 中 `cursor.All` 一次性把所有结果载入内存。对结果集较大的场景没有暴露游标式 API。可以补充 `Each(ctx, fn func(decode func(any) error) error) error` 或返回 `*mongo.Cursor`
- 没有提供 `FindOne` 的"未匹配 → 返回 (nil, ErrNoRows)"语义封装，调用方需要直接判断 `mongo.ErrNoDocuments`。Knex 的 `.first()` 在无匹配时返回 undefined，更友好；可以提供 `FirstOrNil` 之类的封装

#### Update

```go
func (q *Query) Update(ctx context.Context, update interface{}) (int64, error) {
    ...
    result, err := q.collection.UpdateMany(ctx, q.buildFilter(), normalizeUpdate(update))
    ...
    return result.ModifiedCount, nil
}

func normalizeUpdate(update interface{}) interface{} {
    doc, ok := update.(bson.M)
    if !ok {
        return update
    }
    for key := range doc {
        if strings.HasPrefix(key, "$") {
            return update
        }
    }
    return bson.M{"$set": doc}
}
```

亮点：自动把不含 `$` 操作符的 `bson.M` 包装成 `$set`，对调用方非常友好，是 Knex 风格更新的精髓。

不足：

- 只识别 `bson.M`。如果调用方传 `bson.D`、`map[string]interface{}` 或结构体，逻辑直接绕过包装，最终会被 driver 拒收（驱动要求更新文档必须包含操作符），调用方拿到的错误信息不够友好
- 缺少 `UpdateOne` / `Upsert` 的能力。建议补充：
  - `UpdateFirst(ctx, doc)`（对应 `UpdateOne`）
  - `Upsert(ctx, doc)`（设置 `options.Update().SetUpsert(true)`）
  - 返回值除 `ModifiedCount` 外，还可暴露 `MatchedCount` 和 `UpsertedID`，否则上层无法判断是否真的命中
- 当前 `Update` 在条件为空时会更新整个集合，没有保护。强烈建议加上"当 conditions 与 orGroups 都为空时直接报错"，避免误操作（Knex 也有 `update without where` 的告警机制可借鉴）

#### Delete

```go
result, err := q.collection.DeleteMany(ctx, q.buildFilter())
```

与 Update 同样的安全隐患：空条件会清空整张集合。建议加入"空过滤器拒绝"开关，或单独提供 `Truncate(ctx)` 表达清空意图。

#### Count / Exists

```go
func (q *Query) Exists(ctx context.Context) (bool, error) {
    count, err := q.Count(ctx)
    return count > 0, err
}
```

`Exists` 通过 `Count` 实现，简单可靠。性能上更优的实现是 `FindOne` 加上 `limit=1` 与 `projection={_id:1}`，避免全表计数。当集合很大时差异显著。可以保留两种实现，让调用方按场景选择。

#### EnsureIndexes / DropCollection

封装薄、语义清晰，没有问题。`EnsureIndexes` 在 `indexes` 为空时返回 `nil, nil`，避免不必要的驱动调用。

#### requireCollection

```go
func (q *Query) requireCollection() error {
    if q == nil || q.collection == nil {
        return ErrCollectionRequired
    }
    return nil
}
```

通用守卫，被所有执行方法调用，是良好的工程实践。

### 2.5 query_aggregate.go

```go
pipeline := bson.A{
    bson.M{"$match": q.buildFilter()},
    bson.M{"$group": bson.M{"_id": nil, "result": accumulator}},
}
```

设计清晰，复用 `buildFilter()`。需要注意：

- 返回类型统一为 `float64`，对整型字段也能工作（MongoDB 会做数值提升）；但调用方拿到的精度可能与字段原类型不一致，文档需要提示
- `$max`、`$min` 仅适用于数值字段；对字符串、时间字段不适用，会返回字段的字典序最大值或最小值，但因解码到 `float64` 会失败。文档应当注明"仅适用于数值字段"，或将返回类型改为 `interface{}` 让调用方自行处理
- `cursor.All(ctx, &results)` 后 `defer cursor.Close(ctx)` 顺序正常
- `Distinct` 直接调用 driver 的 `Distinct`，返回 `[]interface{}`。这是 driver 行为的简单透传，OK
- `Pluck` 通过修改 `q.fields` 后调用 `Find` 实现，副作用是污染了原 Query 的投影状态。如果调用方在 `Pluck` 之后再调用 `Find`，会得到只含单字段的结果。建议在 `Pluck` 内创建一份 Query 副本

### 2.6 filter.go

整个过滤器的翻译核心。逐项审视：

#### opMap

```go
var opMap = map[string]string{
    "!=":   "$ne",
    "<>":   "$ne",
    ">":    "$gt", ">=":   "$gte",
    "<":    "$lt", "<=":   "$lte",
    "$in":  "$in",
    "$nin": "$nin",
}
```

支持 SQL 风格与 MongoDB 原生操作符并存，灵活。但缺失常用的：

- `like` / `ilike`（应翻译为 `$regex` 或正则带 `i`）
- `regex`（直接 `$regex`）
- `exists`（`$exists`）
- `size`（`$size`）

对齐 Knex 后续可补充。

#### applyCondition

```go
case "between":
    pair := c.value.([2]interface{})
    mergeFieldOp(filter, c.field, "$gte", pair[0])
    mergeFieldOp(filter, c.field, "$lte", pair[1])
```

直接类型断言到 `[2]interface{}`，调用方只能通过 `WhereBetween` 间接进入，目前安全。但是该 case 用 `panic` 风险防御不足，建议增加 `ok` 检查并回退。

```go
default:
    mongoOp, ok := opMap[c.op]
    if !ok {
        mongoOp = c.op
    }
    mergeFieldOp(filter, c.field, mongoOp, c.value)
```

兜底逻辑允许传入任意操作符（如 `$regex`），灵活性高。但同时也吃掉了拼写错误（如 `>==` 会被原样下发到 MongoDB 引发驱动错误）。是设计权衡，文档中需要明示。

#### mergeFieldOp

```go
existing, ok := filter[field]
if ok {
    if m, isMap := existing.(bson.M); isMap {
        m[op] = value
        return
    }
}
filter[field] = bson.M{op: value}
```

合并多个操作符到同字段的能力，被测试用例 `TestBuildFilterMergesMultipleOpsOnSameField` 覆盖。但是当字段已存在一个等值条件（`field: 18`），再 `Where(field, ">", 10)` 时，原值会被新的 `bson.M{"$gt":10}` 直接覆盖。这是悄无声息的行为，可能不符合调用方预期。修复建议：

- 遇到 existing 是非 map 时，把它转成 `{"$eq": existing}` 再合并，或者返回错误

#### buildFilter 的 $or 拼装

```go
if len(q.orGroups) > 0 {
    orClauses := []bson.M{filter}
    for _, group := range q.orGroups {
        clause := bson.M{}
        for _, c := range group {
            applyCondition(clause, c)
        }
        orClauses = append(orClauses, clause)
    }
    return bson.M{"$or": orClauses}
}
```

逻辑：把主链条件作为 `$or` 的第一个分支，每个 `OrWhere` 是一个独立分支。但这与 SQL 语义有偏差：

- SQL：`A AND B OR C` 通常解析为 `(A AND B) OR C`
- 本实现：`Where(A).Where(B).OrWhere(C)` 会被翻译为 `$or: [{A,B}, {C}]`，即 `(A AND B) OR C`，与 SQL 一致

但 Knex 的语义是 `.where(A).where(B).orWhere(C)` 对应 `WHERE A AND B OR C`，由数据库的运算符优先级决定。本实现"用主链作为第一个分支"恰好与 Knex 直觉一致，是个不错的选择。

风险：当主链为空（没有任何 Where）只有 OrWhere 时，第一个分支会是空对象 `{}`，MongoDB 中 `{}` 匹配所有文档，会让整个 `$or` 退化为全表匹配。需要在 `buildFilter` 中过滤掉空的 base clause：

```go
orClauses := []bson.M{}
if len(filter) > 0 {
    orClauses = append(orClauses, filter)
}
```

#### buildProjection

```go
func (q *Query) buildProjection() bson.M {
    if len(q.fields) == 0 {
        return nil
    }
    proj := bson.M{}
    for _, f := range q.fields {
        proj[f] = 1
    }
    return proj
}
```

只支持 inclusive。当用户希望排除 `_id` 时无法表达。可补充 `_id: 0` 的便捷方法，或允许 `Select("name", "-_id")` 这类语法糖。

### 2.7 naming.go

#### CollectionName

```go
typ := reflect.TypeOf(value)
for typ != nil && typ.Kind() == reflect.Pointer {
    typ = typ.Elem()
}
if typ == nil {
    return ""
}
if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
    typ = typ.Elem()
    for typ.Kind() == reflect.Pointer {
        typ = typ.Elem()
    }
}
if typ.Kind() != reflect.Struct {
    return ""
}
return pluralize(toSnake(typ.Name()))
```

正确处理了 pointer、slice、array、pointer slice 等组合。但是：

- 不识别 `TableName() string` 这种自定义接口。对齐 GORM/Knex 实践，应允许结构体通过实现 `CollectionNamer interface { CollectionName() string }` 覆盖默认推导
- 嵌入结构体场景未测试，但因为只取 `typ.Name()`，匿名嵌入不会带来名称污染

#### toSnake

```go
for i, r := range value {
    if i > 0 && r >= 'A' && r <= 'Z' {
        out.WriteByte('_')
    }
    out.WriteRune(r)
}
return strings.ToLower(out.String())
```

简易实现。对连续大写处理不佳：`HTTPServer` 会被转成 `h_t_t_p_server`，而非更直观的 `http_server`。考虑用滑窗算法（前一字符大写、当前字符大写、下一字符小写 → 插入下划线）改进。

#### pluralize

实现了 4 类规则：

- `y` 前是辅音 → `ies`
- 结尾 `s/x/sh/ch` → `es`
- 其他 → `s`

测试覆盖了 11 个用例。注意以下边角情况未处理：

- 不规则复数：`person/people`, `child/children`, `man/men`
- 已是复数的名称：`users` 会被再加 `es` 变成 `userses`
- `o` 结尾：`hero` 实际应为 `heroes`，会被当成普通名词加 `s`

对于内部使用够用，但对齐 Knex 的命名约定建议接入更完整的复数化库（如 `github.com/jinzhu/inflection`），或者要求所有调用方显式实现 `CollectionName()`。

### 2.8 log.go

#### 实现简评

```go
errorLogger = log.New(os.Stdout, "\033[31m[error]\033[0m ", log.LstdFlags|log.Lshortfile)
infoLogger  = log.New(os.Stdout, "\033[34m[info ]\033[0m ", log.LstdFlags|log.Lshortfile)
```

直接使用 ANSI 颜色码，在不支持颜色的终端（如 Windows 默认 cmd、CI 日志收集器）中会显示乱码前缀。建议：

- 通过环境变量或 Option 控制是否启用颜色
- 或使用 `github.com/fatih/color` 等库自动检测 TTY

```go
var (
    Error  = errorLogger.Println
    Errorf = errorLogger.Printf
    Info   = infoLogger.Println
    Infof  = infoLogger.Printf
)
```

将函数值暴露为变量，调用方能直接 `lark_orm.Info(...)`。但这导致调用栈中 `log.Lshortfile` 永远指向 `log.go` 而非调用现场，调试体验差。修复办法：把日志输出封装成函数并使用 `log.Output(2, ...)`。

#### SetLevel

```go
func SetLevel(level int) {
    mu.Lock()
    defer mu.Unlock()
    for _, logger := range loggers {
        logger.SetOutput(os.Stdout)
    }
    if ErrorLevel < level {
        errorLogger.SetOutput(io.Discard)
    }
    if InfoLevel < level {
        infoLogger.SetOutput(io.Discard)
    }
}
```

逻辑正确但常量定义有些反直觉：

```go
InfoLevel = 0   // 最低级别（最详尽）
ErrorLevel = 1
Disabled = 2
```

`SetLevel(InfoLevel)` 表示打开全部日志；`SetLevel(ErrorLevel)` 表示只打错误及以上；`SetLevel(Disabled)` 关闭所有。这是常见做法，但建议在常量上方加一行注释解释"数字越大日志越少"。

另外，整个 lark_orm 模块内部目前完全没有调用 `Info/Error`，这套日志器实际是 "对外暴露但内部不使用"。如果将来要加慢查询日志、查询失败日志、事务回滚日志，这套基础设施已经准备好。

### 2.9 lark_orm_test.go

测试覆盖较为充分，亮点：

- `openTestEngine` 使用纳秒时间戳生成隔离数据库，避免互相污染
- 在每个用例后通过 `t.Cleanup` 自动 `DropDatabase`
- 通过 `__access_check` 探测权限，遇到鉴权失败自动 `t.Skipf`，对 CI 友好
- 同时包含集成测试（依赖真实 MongoDB）与纯单元测试（`TestBuildFilter*`、`TestPluralize`、`TestNormalizeUpdate` 等）

可改进点：

- `openTestEngine` 中 `engine.Collection("users").Insert(ctx, ...)` 多处忽略错误，集成测试失败时定位较难。建议改用 `require.NoError` 模式或显式 `t.Fatal`
- 缺少事务（`Transaction`）的端到端测试。当前测试不覆盖 `engine.go` 中最复杂的方法之一
- 缺少 nil engine 调用 `NextSequence/Transaction` 的测试（虽然源码做了 nil 守卫）
- 没有针对"`OrWhere` 与空主链组合产生空 base clause"的回归测试，正好对应 2.6 节提到的潜在 bug
- 没有覆盖 `Pluck` 修改 `q.fields` 后的副作用
- 没有覆盖大小写混合的 `OrderBy` 方向
- 表测试用 `map[string]interface{}` 作为输入是常见 Go 反模式（map 迭代无序，错误信息不可复现），如 `TestCollectionName` 使用了这种结构，可改为 `[]struct{ name string; input any; want string }`

## 三、跨文件横切问题

### 3.1 链式构建器的可复制性

如 2.3 节所述，`*Query` 通过 append 修改自身切片，与 Knex 的语义有差异。生产中如果 SOA 把同一 base query 分叉使用，会出现非常隐蔽的 bug。这是最值得优先处理的设计问题。

### 3.2 错误处理风格

- 多处使用 `errors.New(...)` 给出短文本，没有包名前缀。建议统一前缀为 `lark_orm: `，便于日志中定位错误来源
- 没有使用 `%w` 包裹底层 driver 错误。调用方无法 `errors.As(err, *mongo.CommandError{})` 判断错误类型
- 哨兵错误 `ErrCollectionRequired` 是唯一对外可比对的错误，建议补充：
  - `ErrEngineNotInitialized`
  - `ErrSequenceNameRequired`
  - `ErrEmptyFilterRejected`（用于 update/delete 守卫）

### 3.3 安全性

- `Update` / `Delete` 空过滤器会作用于全集合，没有显式开关。生产事故的高风险点
- 没有 `context.Context` 默认超时。所有执行方法依赖调用方传入带超时的 ctx，若调用方传 `context.Background()` 会无限等待
- `NextSequence` 在并发下会争抢同一文档（`counters/{_id:name}`），原子性由 MongoDB 保证，但缺少 retry/backoff，瞬时压力大时返回错误率可能上升

### 3.4 可观测性

- 没有 hook 机制（`before/after query` 回调）
- 没有慢查询埋点
- log.go 准备就绪但内部无调用
- 没有 OpenTelemetry / metrics 接入点

### 3.5 类型安全

- 全部基于 `interface{}`，编译期无法发现传错字段或类型
- 缺少基于泛型的 typed Query。Go 1.26 已经支持泛型多年，可以考虑：

```go
type TypedQuery[T any] struct { *Query }
func Model[T any](e *Engine) *TypedQuery[T]
func (q *TypedQuery[T]) Find(ctx) ([]T, error)
```

不会破坏现有 API，新旧并存即可。

### 3.6 与 Knex API 的对齐差距

| Knex 能力                                    | lark_orm 现状                                                   |
| -------------------------------------------- | --------------------------------------------------------------- |
| `.where()` / `.andWhere()` / `.orWhere()`    | 已对齐 `Where` / `OrWhere`，缺 `andWhere`（实际等价于 `Where`） |
| 回调式分组 `where(qb => {})`                 | 未支持                                                          |
| `.whereExists`                               | 未支持                                                          |
| `.whereRaw`                                  | 未支持（MongoDB 场景可对应"接收任意 `bson.M` 注入"）            |
| `.join` 等                                   | MongoDB 无关系，可用 `$lookup`，未封装                          |
| `.insert` 返回 ID                            | 已支持                                                          |
| `.update` 返回受影响行数                     | 已支持，但仅 ModifiedCount                                      |
| `.del` / `.delete`                           | 已支持                                                          |
| `.first`                                     | 已支持                                                          |
| `.pluck`                                     | 已支持但有副作用                                                |
| `.count` / `.sum` / `.avg` / `.max` / `.min` | 已支持                                                          |
| `.distinct`                                  | 已支持                                                          |
| `.groupBy`                                   | 未支持（聚合都只 `_id: nil`）                                   |
| `.having`                                    | 未支持                                                          |
| `.raw` / 子查询                              | 未支持                                                          |
| 事务                                         | 已支持                                                          |
| 迁移 / 种子                                  | 未支持（MongoDB schema-less，重要性较低）                       |

## 四、优化建议清单（按优先级）

P0 严重 / 安全：

1. `Update` / `Delete` 在空过滤器时返回错误或要求显式 `AllowEmptyFilter()` 开关
2. `buildFilter` 在主链为空时，不把 `{}` 作为 `$or` 的第一个分支
3. `mergeFieldOp` 在已有非 map 等值条件时，提示冲突或正确合并为 `$eq`
4. 链式 Query 在分叉使用时的切片共享问题（提供 `Clone` 或防御性拷贝）

P1 体验 / 可维护：

5. 哨兵错误集中到 `lark_orm.go` 或 `errors.go`，全部使用 `lark_orm: ` 前缀，并 `%w` 包裹底层错误
6. `Pluck` 内部使用 Query 副本，避免污染原 Query 的 `fields`
7. `OrderBy` 使用 `strings.EqualFold` 处理方向大小写
8. `NewEngine` 引入函数式选项（`WithClientOptions`、`WithSequenceCollection` 等）
9. 集合自定义命名：识别 `interface { CollectionName() string }`
10. `Update` 支持 `UpdateFirst`（UpdateOne）、`Upsert`，返回 `MatchedCount / UpsertedID`

P2 功能扩展：

11. 添加 `Exists` 的 FindOne 优化版本
12. 增加 `WhereLike`、`WhereExists`、`WhereRegex`
13. 增加 `GroupBy` + 多聚合的高阶 API
14. 增加迭代/游标式 API（处理大结果集）
15. 接入 OpenTelemetry 与 hook 机制，开启慢查询日志
16. 引入泛型 `TypedQuery[T]`，提升类型安全

P3 工程化：

17. `toSnake` 修复连续大写 / 缩写场景
18. `pluralize` 接入完整复数化库或允许自定义
19. 日志默认仅在 TTY 输出颜色
20. 增加 Transaction、Pluck 副作用、空主链 OrWhere 的回归测试
21. `lark_orm.go` 增加包级文档注释

## 五、结论

`lark_orm` 当前是一个克制、聚焦的 MongoDB ORM 实现：核心抽象只有 Engine 与 Query，全部代码量约 600 行（不含测试），认知负担很低，足以应对 lark_demo 这类中等复杂度业务。

它已经良好地实现了 Knex 风格的链式查询 API、自动 `$set` 包装、自增序列、事务、聚合等关键能力，并通过较为完善的集成测试与单元测试保证了正确性。

主要的改进空间集中在四个方向：

1. 安全防御（空 filter 的全集合误操作、链式分叉的切片共享）
2. API 表达力（更多 Where 变体、Upsert、GroupBy、回调式分组、Clone）
3. 可观测性与类型安全（hook、metrics、泛型 typed query）
4. 工程细节（错误包装、日志、命名规则、复数化）

按上述优先级推进，可以逐步把 `lark_orm` 打磨成既贴近 Knex 体验、又契合 Go 习惯的生产级 MongoDB ORM。
