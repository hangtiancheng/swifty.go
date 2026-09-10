// Package lua holds the Lua scripts used to make compound lock operations
// atomic on the Redis side.
package lua

// LuaCheckAndDeleteDistributedLock deletes the lock only when the caller owns
// it. KEYS[1] is the lock key, ARGV[1] is the caller's token. It returns 1
// when the lock was deleted and 0 otherwise.
const LuaCheckAndDeleteDistributedLock = `
  local lockKey = KEYS[1]
  local targetToken = ARGV[1]
  local currentToken = redis.call('get', lockKey)
  if not currentToken or currentToken ~= targetToken then
    return 0
  else
    return redis.call('del', lockKey)
  end
`

// LuaCheckAndExpireDistributedLock extends the expiry of the lock only when
// the caller owns it. KEYS[1] is the lock key, ARGV[1] is the caller's token
// and ARGV[2] is the new expiry in seconds. It returns 1 when the lock was
// extended and 0 otherwise.
const LuaCheckAndExpireDistributedLock = `
  local lockKey = KEYS[1]
  local targetToken = ARGV[1]
  local duration = ARGV[2]
  local currentToken = redis.call('get', lockKey)
  if not currentToken or currentToken ~= targetToken then
    return 0
  else
    return redis.call('expire', lockKey, duration)
  end
`
