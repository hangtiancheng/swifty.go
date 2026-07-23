module github.com/hangtiancheng/swifty.go/demo/consistent_hash

go 1.26

require (
	github.com/demdxx/gocast v1.2.0
	github.com/gomodule/redigo v1.9.3
	github.com/hangtiancheng/swifty.go/demo/redis_lock v0.0.0
	github.com/spaolacci/murmur3 v1.1.0
)

require github.com/pkg/errors v0.9.2-0.20201214064552-5dd12d0cfe7f // indirect

replace github.com/hangtiancheng/swifty.go/demo/redis_lock => ../redis_lock
