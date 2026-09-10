package timewheel

const (
	// LuaAddTasks adds a task. When the task key is flagged as deleted in the
	// delete set, the flag is cleared first (a re-added task is no longer
	// marked for deletion). The zset shard key (KEYS[1]) is derived from the
	// minute the task belongs to; the score is the unix timestamp of the
	// execution time.
	LuaAddTasks = `
	   local zsetKey = KEYS[1]
	   local deleteSetKey = KEYS[2]
	   local score = ARGV[1]
	   local task = ARGV[2]
	   local taskKey = ARGV[3]
	   redis.call('srem',deleteSetKey,taskKey)
	   return redis.call('zadd',zsetKey,score,task)
	`

	// LuaDeleteTask flags a task key as deleted by adding it to the delete
	// set. The set expires after 120 seconds so abandoned sets do not leak.
	LuaDeleteTask = `
	   local deleteSetKey = KEYS[1]
	   local taskKey = ARGV[1]
	   redis.call('sadd',deleteSetKey,taskKey)
	   local scnt = redis.call('scard',deleteSetKey)
	   if (tonumber(scnt) == 1)
	   then
	       redis.call('expire',deleteSetKey,120)
       end
	   return scnt
	`

	// LuaZrangeTasks fetches every task whose score falls into the given
	// range, excluding the ones flagged as deleted, and removes the fetched
	// entries from the zset so each task is fetched at most once. The reply
	// is a single array: element 1 holds the delete set members, the
	// remaining elements hold the serialized tasks.
	LuaZrangeTasks = `
	   local zsetKey = KEYS[1]
	   local deleteSetKey = KEYS[2]
	   local score1 = ARGV[1]
	   local score2 = ARGV[2]
	   local deleteSet = redis.call('smembers',deleteSetKey)
	   local targets = redis.call('zrange',zsetKey,score1,score2,'byscore')
	   redis.call('zremrangebyscore',zsetKey,score1,score2)
	   local reply = {}
	   reply[1] = deleteSet
	   for i, v in ipairs(targets) do
	       reply[#reply+1]=v
	   end
       return reply
	`
)
