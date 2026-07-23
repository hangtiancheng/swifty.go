module github.com/hangtiancheng/swifty.go/demo/consistent_hash

go 1.26

require (
	github.com/hangtiancheng/swifty.go/demo/redis_lock v0.0.0
	github.com/redis/go-redis/v9 v9.21.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/hangtiancheng/swifty.go/demo/redis_lock => ../redis_lock
