package redis

import "github.com/redis/go-redis/v9"

// newScript wraps a Lua script; the first execution uses EVAL and subsequent
// executions use the cached EVALSHA digest.
func newScript(src string) *redis.Script {
	return redis.NewScript(src)
}
