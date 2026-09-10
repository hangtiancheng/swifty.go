// Package redis provides a HashRing implementation that keeps the ring in a
// Redis sorted set. All Redis access goes through the redis_lock client, so
// the connection pool and the distributed ring lock are shared.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hangtiancheng/swifty.go/apps/redis_lock"
)

// ScoreEntity is a member/score pair read out of the sorted set that stores
// the hash ring.
type ScoreEntity struct {
	Score int64
	Val   string
}

// RedisHashRing is a HashRing implementation backed by a Redis sorted set.
// The sorted set members are JSON arrays of raw virtual node keys and the
// scores are the positions of the virtual nodes on the ring.
type RedisHashRing struct {
	key         string
	redisClient *redis_lock.Client
}

// NewRedisHashRing builds a Redis backed hash ring identified by key. The
// redisClient is also used for the distributed ring lock.
func NewRedisHashRing(key string, redisClient *redis_lock.Client) *RedisHashRing {
	return &RedisHashRing{
		key:         key,
		redisClient: redisClient,
	}
}

func (r *RedisHashRing) getLockKey() string {
	return fmt.Sprintf("redis:consistent_hash:ring:lock:%s", r.key)
}

func (r *RedisHashRing) getTableKey() string {
	return fmt.Sprintf("redis:consistent_hash:ring:%s", r.key)
}

func (r *RedisHashRing) getNodeReplicaKey() string {
	return fmt.Sprintf("redis:consistent_hash:ring:node:replica:%s", r.key)
}

func (r *RedisHashRing) getNodeDataKey(nodeID string) string {
	return fmt.Sprintf("redis:consistent_hash:ring:node:data:%s", nodeID)
}

// Lock acquires the distributed ring lock, expiring after expireSeconds.
func (r *RedisHashRing) Lock(ctx context.Context, expireSeconds int) error {
	lock := redis_lock.NewRedisLock(r.getLockKey(), r.redisClient, redis_lock.WithExpireSeconds(int64(expireSeconds)))
	return lock.Lock(ctx)
}

// Unlock releases the distributed ring lock.
func (r *RedisHashRing) Unlock(ctx context.Context) error {
	lock := redis_lock.NewRedisLock(r.getLockKey(), r.redisClient)
	return lock.Unlock(ctx)
}

// Add registers a raw virtual node key under the given score.
func (r *RedisHashRing) Add(ctx context.Context, score int32, nodeID string) error {
	reply, err := r.eval(ctx, luaZAddNodeID, r.getTableKey(), strconv.FormatInt(int64(score), 10), nodeID)
	if err != nil {
		return fmt.Errorf("redis ring add failed: %w", err)
	}

	switch code, _ := replyToInt64(reply); code {
	case 1, 0:
		return nil
	case -1:
		return fmt.Errorf("redis ring add failed, several members share score: %d", score)
	default:
		return fmt.Errorf("redis ring add failed, unexpected reply: %v", reply)
	}
}

// Ceiling returns the smallest score >= score, wrapping around to the first
// score of the ring when the requested score is beyond the last one. It
// returns -1 when the ring is empty.
func (r *RedisHashRing) Ceiling(ctx context.Context, score int32) (int32, error) {
	entities, err := r.zrange(ctx, luaZRangeCeiling, strconv.FormatInt(int64(score), 10))
	if err != nil {
		return 0, fmt.Errorf("redis ring ceiling failed: %w", err)
	}
	if len(entities) > 0 {
		return int32(entities[0].Score), nil
	}

	// Wrap around: fall back to the first score of the ring.
	if entities, err = r.zrange(ctx, luaZRangeFirstOrLast, "first"); err != nil {
		return 0, fmt.Errorf("redis ring ceiling failed: %w", err)
	}
	if len(entities) > 0 {
		return int32(entities[0].Score), nil
	}

	return -1, nil
}

// Floor returns the largest score <= score, wrapping around to the last score
// of the ring when the requested score is before the first one. It returns -1
// when the ring is empty.
func (r *RedisHashRing) Floor(ctx context.Context, score int32) (int32, error) {
	entities, err := r.zrange(ctx, luaZRangeFloor, strconv.FormatInt(int64(score), 10))
	if err != nil {
		return 0, fmt.Errorf("redis ring floor failed: %w", err)
	}
	if len(entities) > 0 {
		return int32(entities[0].Score), nil
	}

	// Wrap around: fall back to the last score of the ring.
	if entities, err = r.zrange(ctx, luaZRangeFirstOrLast, "last"); err != nil {
		return 0, fmt.Errorf("redis ring floor failed: %w", err)
	}
	if len(entities) > 0 {
		return int32(entities[0].Score), nil
	}

	return -1, nil
}

// Rem removes a raw virtual node key from the given score.
func (r *RedisHashRing) Rem(ctx context.Context, score int32, nodeID string) error {
	reply, err := r.eval(ctx, luaZRemNodeID, r.getTableKey(), strconv.FormatInt(int64(score), 10), nodeID)
	if err != nil {
		return fmt.Errorf("redis ring rem failed: %w", err)
	}

	switch code, _ := replyToInt64(reply); code {
	case 1, 0:
		return nil
	case -1:
		return fmt.Errorf("redis ring rem failed, score: %d not exist", score)
	case -2:
		return fmt.Errorf("redis ring rem failed, several members share score: %d", score)
	default:
		return fmt.Errorf("redis ring rem failed, unexpected reply: %v", reply)
	}
}

// Nodes returns the mapping from node id to virtual node count.
func (r *RedisHashRing) Nodes(ctx context.Context) (map[string]int, error) {
	reply, err := r.eval(ctx, luaHGetAll, r.getNodeReplicaKey())
	if err != nil {
		return nil, fmt.Errorf("redis ring nodes hgetall failed: %w", err)
	}

	flat, err := replyToStringSlice(reply)
	if err != nil {
		return nil, fmt.Errorf("redis ring nodes hgetall failed: %w", err)
	}

	nodes := make(map[string]int, len(flat)/2)
	for i := 0; i+1 < len(flat); i += 2 {
		replicas, err := strconv.Atoi(flat[i+1])
		if err != nil {
			return nil, fmt.Errorf("redis ring nodes failed, invalid replica count %q: %w", flat[i+1], err)
		}
		nodes[flat[i]] = replicas
	}
	return nodes, nil
}

// AddNodeToReplica records the number of virtual nodes of a node, marking the
// node as registered on the ring.
func (r *RedisHashRing) AddNodeToReplica(ctx context.Context, nodeID string, replicas int) error {
	if _, err := r.eval(ctx, luaHSetField, r.getNodeReplicaKey(), nodeID, strconv.Itoa(replicas)); err != nil {
		return fmt.Errorf("redis ring add node to replica failed: %w", err)
	}
	return nil
}

// DeleteNodeToReplica drops the registration of a node.
func (r *RedisHashRing) DeleteNodeToReplica(ctx context.Context, nodeID string) error {
	if _, err := r.eval(ctx, luaHDelField, r.getNodeReplicaKey(), nodeID); err != nil {
		return fmt.Errorf("redis ring delete node to replica failed: %w", err)
	}
	return nil
}

// Node returns the raw virtual node keys registered under the given score.
func (r *RedisHashRing) Node(ctx context.Context, score int32) ([]string, error) {
	scoreStr := strconv.FormatInt(int64(score), 10)
	entities, err := r.zrange(ctx, luaZRangeByScore, scoreStr, scoreStr)
	if err != nil {
		return nil, fmt.Errorf("redis ring node zrange by score failed: %w", err)
	}

	if len(entities) != 1 {
		return nil, fmt.Errorf("redis ring node failed, invalid score entity count: %d", len(entities))
	}

	var nodeIDs []string
	if err = json.Unmarshal([]byte(entities[0].Val), &nodeIDs); err != nil {
		return nil, fmt.Errorf("redis ring node failed, decode member %q: %w", entities[0].Val, err)
	}

	return nodeIDs, nil
}

// DataKeys returns the data keys currently assigned to the node. A node
// without recorded keys yields an empty set.
func (r *RedisHashRing) DataKeys(ctx context.Context, nodeID string) (map[string]struct{}, error) {
	reply, err := r.eval(ctx, luaGetOrNil, r.getNodeDataKey(nodeID))
	if err != nil {
		return nil, fmt.Errorf("redis ring data keys get failed: %w", err)
	}

	raw, err := replyToString(reply)
	if err != nil {
		return nil, fmt.Errorf("redis ring data keys get failed: %w", err)
	}
	// The empty string marks a missing key, i.e. no recorded data keys.
	if raw == "" {
		return map[string]struct{}{}, nil
	}

	return decodeDataKeys(raw)
}

// AddNodeToDataKeys merges data keys into the set assigned to the node.
func (r *RedisHashRing) AddNodeToDataKeys(ctx context.Context, nodeID string, dataKeys map[string]struct{}) error {
	args := make([]string, 0, len(dataKeys))
	for dataKey := range dataKeys {
		args = append(args, dataKey)
	}

	if _, err := r.eval(ctx, luaAddDataKeys, r.getNodeDataKey(nodeID), args...); err != nil {
		return fmt.Errorf("redis ring add node to data keys failed: %w", err)
	}
	return nil
}

// DeleteNodeToDataKeys removes data keys from the set assigned to the node.
// Removing keys of an untracked node is a no-op.
func (r *RedisHashRing) DeleteNodeToDataKeys(ctx context.Context, nodeID string, dataKeys map[string]struct{}) error {
	args := make([]string, 0, len(dataKeys))
	for dataKey := range dataKeys {
		args = append(args, dataKey)
	}

	if _, err := r.eval(ctx, luaDeleteDataKeys, r.getNodeDataKey(nodeID), args...); err != nil {
		return fmt.Errorf("redis ring delete node to data keys failed: %w", err)
	}
	return nil
}

// eval runs a Lua script through the redis_lock client. The script receives
// exactly one key followed by its arguments.
func (r *RedisHashRing) eval(ctx context.Context, script, key string, args ...string) (interface{}, error) {
	keysAndArgs := make([]interface{}, 0, len(args)+1)
	keysAndArgs = append(keysAndArgs, key)
	for _, arg := range args {
		keysAndArgs = append(keysAndArgs, arg)
	}
	return r.redisClient.Eval(ctx, script, 1, keysAndArgs)
}

// zrange runs a sorted set query script and decodes its [member, score, ...]
// reply.
func (r *RedisHashRing) zrange(ctx context.Context, script string, args ...string) ([]*ScoreEntity, error) {
	reply, err := r.eval(ctx, script, r.getTableKey(), args...)
	if err != nil {
		return nil, err
	}
	return replyToScoreEntities(reply)
}

// decodeDataKeys parses the JSON object stored under a node's data key. The
// object values are placeholders; only the key set matters.
func decodeDataKeys(raw string) (map[string]struct{}, error) {
	var values map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("redis ring data keys decode failed: %w", err)
	}

	dataKeys := make(map[string]struct{}, len(values))
	for dataKey := range values {
		dataKeys[dataKey] = struct{}{}
	}
	return dataKeys, nil
}

// replyToScoreEntities converts a flat [member, score, ...] Lua reply into
// ScoreEntity values. A nil reply yields an empty result.
func replyToScoreEntities(reply interface{}) ([]*ScoreEntity, error) {
	if reply == nil {
		return nil, nil
	}

	raws, ok := reply.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected redis reply type: %T", reply)
	}
	if len(raws)%2 != 0 {
		return nil, fmt.Errorf("invalid redis reply length: %d", len(raws))
	}

	entities := make([]*ScoreEntity, 0, len(raws)/2)
	for i := 0; i < len(raws); i += 2 {
		val, err := replyToString(raws[i])
		if err != nil {
			return nil, err
		}
		score, err := replyToInt64(raws[i+1])
		if err != nil {
			return nil, err
		}
		entities = append(entities, &ScoreEntity{Score: score, Val: val})
	}
	return entities, nil
}

// replyToStringSlice converts a flat Lua reply into a string slice.
func replyToStringSlice(reply interface{}) ([]string, error) {
	if reply == nil {
		return nil, nil
	}

	raws, ok := reply.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected redis reply type: %T", reply)
	}

	out := make([]string, 0, len(raws))
	for _, raw := range raws {
		s, err := replyToString(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// replyToString converts a single Lua reply element into a string.
func replyToString(reply interface{}) (string, error) {
	switch v := reply.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("unexpected redis reply type: %T", reply)
	}
}

// replyToInt64 converts a single Lua reply element into an int64.
func replyToInt64(reply interface{}) (int64, error) {
	switch v := reply.(type) {
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case []byte:
		return strconv.ParseInt(string(v), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis reply type: %T", reply)
	}
}
