- consistent_cache
- redis_lock
- raft_demo
- red_mq
- consistent_hash
- lsm_tree
- redis_demo (doing)

替换所有的中文为专业的英文, 替换 github.com/panjf2000/ants 为 github.com/bytedance/gopkg@v0.1.4/util/gopool; 彻底移除 github.com/spf13/cast、github.com/stretchr/testify (使用 go 原生 test)、彻底移除 go.uber.org/zap、gopkg.in/natefinch/lumberjack.v2 (使用 go 原生 log)

- time_wheel

替换所有的中文为专业的英文, 彻底移除 github.com/demdxx/gocast、替换 github.com/gomodule/redigo 为 github.com/redis/go-redis/v9

- timer_demo

替换所有的中文为专业的英文
替换 github.com/gin-gonic/gin 为 go work 下的 github.com/hangtiancheng/swifty.go/swifty_http (调用 /swifty_http 技能)
彻底移除 github.com/go-sql-driver/mysql
替换 github.com/gomodule/redigo 为 github.com/redis/go-redis/v9
彻底移除 github.com/gorhill/cronexpr
替换 github.com/panjf2000/ants 为 github.com/bytedance/gopkg@v0.1.4/util/gopool
彻底移除 github.com/spaolacci/murmur3 (使用 go 原生 hash)
替换 github.com/spf13/viper 为 gopkg.in/yaml.v3
彻底移除 github.com/swaggo/gin-swagger, 不需要 api 文档
彻底移除 go.uber.org/zap、gopkg.in/natefinch/lumberjack.v2 (使用 go 原生 log)

- tcc_demo

彻底移除 go.uber.org/zap、gopkg.in/natefinch/lumberjack.v2 (使用 go 原生 log)
彻底移除 github.com/stretchr/testify (使用 go 原生 test)
彻底移除 github.com/spf13/cast
彻底移除 github.com/demdxx/gocast
替换 github.com/agiledragon/gomonkey/v2 为 https://github.com/uber-go/mock
