package redis

// The redis backed ring only talks to Redis through the redis_lock client,
// whose command surface is intentionally small (Get/Set/Del/Eval/...). Every
// ring operation below is therefore expressed as a small Lua script that runs
// atomically on the server through Client.Eval. Each script receives exactly
// one key via KEYS[1] plus its arguments via ARGV.

// luaZAddNodeID registers a raw virtual node key under the given score.
// Scripts operate on the JSON array of node keys stored as the sorted set
// member of that score.
// ARGV: [score, nodeID]
// Returns 1 when the node key was added, 0 when it was already registered and
// -1 when several members share the score (invalid state).
const luaZAddNodeID = `
local members = redis.call('ZRANGE', KEYS[1], ARGV[1], ARGV[1], 'BYSCORE')
if #members > 1 then
    return -1
end
local nodeIDs = {}
if #members == 1 then
    nodeIDs = cjson.decode(members[1])
    for _, nodeID in ipairs(nodeIDs) do
        if nodeID == ARGV[2] then
            return 0
        end
    end
    redis.call('ZREM', KEYS[1], members[1])
end
table.insert(nodeIDs, ARGV[2])
redis.call('ZADD', KEYS[1], ARGV[1], cjson.encode(nodeIDs))
return 1
`

// luaZRemNodeID removes a raw virtual node key from the given score. The
// member is dropped entirely when no node key is left.
// ARGV: [score, nodeID]
// Returns 1 when the node key was removed, 0 when it was not registered,
// -1 when the score does not exist and -2 when several members share the
// score (invalid state).
const luaZRemNodeID = `
local members = redis.call('ZRANGE', KEYS[1], ARGV[1], ARGV[1], 'BYSCORE')
if #members == 0 then
    return -1
end
if #members > 1 then
    return -2
end
local nodeIDs = cjson.decode(members[1])
local remaining = {}
local found = false
for _, nodeID in ipairs(nodeIDs) do
    if nodeID == ARGV[2] then
        found = true
    else
        table.insert(remaining, nodeID)
    end
end
if not found then
    return 0
end
redis.call('ZREM', KEYS[1], members[1])
if #remaining > 0 then
    redis.call('ZADD', KEYS[1], ARGV[1], cjson.encode(remaining))
end
return 1
`

// luaZRangeByScore returns all [member, score] pairs whose score falls into
// the given inclusive range.
// ARGV: [minScore, maxScore]
const luaZRangeByScore = `
return redis.call('ZRANGE', KEYS[1], ARGV[1], ARGV[2], 'BYSCORE', 'WITHSCORES')
`

// luaZRangeCeiling returns the first [member, score] pair with score >= the
// given score.
// ARGV: [score]
const luaZRangeCeiling = `
return redis.call('ZRANGE', KEYS[1], ARGV[1], '+inf', 'BYSCORE', 'LIMIT', 0, 1, 'WITHSCORES')
`

// luaZRangeFloor returns the first [member, score] pair with score <= the
// given score.
// ARGV: [score]
const luaZRangeFloor = `
return redis.call('ZRANGE', KEYS[1], ARGV[1], '-inf', 'BYSCORE', 'REV', 'LIMIT', 0, 1, 'WITHSCORES')
`

// luaZRangeFirstOrLast returns the [member, score] pair with the smallest
// ("first") or the largest ("last") score of the sorted set.
// ARGV: ["first" | "last"]
const luaZRangeFirstOrLast = `
if ARGV[1] == 'first' then
    return redis.call('ZRANGE', KEYS[1], '-inf', '+inf', 'BYSCORE', 'LIMIT', 0, 1, 'WITHSCORES')
end
return redis.call('ZRANGE', KEYS[1], '+inf', '-inf', 'BYSCORE', 'REV', 'LIMIT', 0, 1, 'WITHSCORES')
`

// luaHSetField stores a field of the node-to-replica-count hash.
// ARGV: [field, value]
const luaHSetField = `
return redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
`

// luaHGetAll returns the flat [field, value, ...] content of a hash.
const luaHGetAll = `
return redis.call('HGETALL', KEYS[1])
`

// luaHDelField removes a field from a hash.
// ARGV: [field]
const luaHDelField = `
return redis.call('HDEL', KEYS[1], ARGV[1])
`

// luaGetOrNil returns the string value of a key, or the empty string when the
// key does not exist. The empty string is the "missing" sentinel: an empty
// value is never stored (only JSON objects with at least one entry are
// written), and using it keeps the check independent of how the underlying
// driver reports nil replies.
const luaGetOrNil = `
local value = redis.call('GET', KEYS[1])
if value == false then
    return ''
end
return value
`

// luaAddDataKeys merges data keys into the JSON object stored under a node's
// data key. The object values are placeholders; only the key set matters.
// ARGV: [dataKey...]
const luaAddDataKeys = `
if #ARGV == 0 then
    return 1
end
local value = redis.call('GET', KEYS[1])
local dataKeys = {}
if value ~= false then
    dataKeys = cjson.decode(value)
end
for _, dataKey in ipairs(ARGV) do
    dataKeys[dataKey] = {}
end
redis.call('SET', KEYS[1], cjson.encode(dataKeys))
return 1
`

// luaDeleteDataKeys removes data keys from the JSON object stored under a
// node's data key and drops the key entirely when no data key is left.
// ARGV: [dataKey...]
const luaDeleteDataKeys = `
local value = redis.call('GET', KEYS[1])
if value == false then
    return 0
end
local dataKeys = cjson.decode(value)
for _, dataKey in ipairs(ARGV) do
    dataKeys[dataKey] = nil
end
local count = 0
for _ in pairs(dataKeys) do
    count = count + 1
end
if count == 0 then
    redis.call('DEL', KEYS[1])
    return 1
end
redis.call('SET', KEYS[1], cjson.encode(dataKeys))
return 1
`
