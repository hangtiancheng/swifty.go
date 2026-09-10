package redis

// LuaCheckEnableAndWriteCache writes the key/value pair with the given expiry
// only when the disable key does not exist.
const LuaCheckEnableAndWriteCache = `
local disable_key = KEYS[1];
local disable_flag = redis.call("get",disable_key);
if disable_flag then
    return 0;
end
local key = KEYS[2];
local value = ARGV[1];
redis.call("set",key,value);
local cache_expire_seconds = tonumber(ARGV[2]);
redis.call("expire",key,cache_expire_seconds);
return 1;
`
