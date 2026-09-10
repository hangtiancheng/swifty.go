package consistent_hash_test

import (
	"context"
	"net"
	"testing"
	"time"

	consistenthash "github.com/hangtiancheng/swifty.go/apps/consistent_hash"
	"github.com/hangtiancheng/swifty.go/apps/consistent_hash/internal/local"
	ciredis "github.com/hangtiancheng/swifty.go/apps/consistent_hash/internal/redis"
	redislock "github.com/hangtiancheng/swifty.go/apps/redis_lock"
)

// TestLocalConsistentHash exercises the full consistent hash flow on top of
// the in-memory skiplist ring. It runs without any external dependency.
func TestLocalConsistentHash(t *testing.T) {
	localHashRing := local.NewSkiplistHashRing()
	loggingMigrator := func(_ context.Context, dataKeys map[string]struct{}, from, to string) error {
		t.Logf("migrating %v from %s to %s", dataKeys, from, to)
		return nil
	}
	consistentHash := consistenthash.NewConsistentHash(
		localHashRing,
		consistenthash.NewFnvHasher(),
		loggingMigrator,
		// Every node owns weight * replicas virtual nodes.
		consistenthash.WithReplicas(5),
		// The ring lock expires automatically after 5 seconds.
		consistenthash.WithLockExpireSeconds(5),
	)
	exerciseConsistentHash(t, consistentHash)
}

// TestRedisConsistentHash runs the same flow on top of the Redis backed ring.
// It requires a Redis instance and skips when none is reachable.
func TestRedisConsistentHash(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 500*time.Millisecond)
	if err != nil {
		t.Skipf("skipping: no redis reachable at 127.0.0.1:6379: %v", err)
	}
	_ = conn.Close()

	redisClient := redislock.NewClient("tcp", "127.0.0.1:6379", "")
	hashRingKey := "consistent_hash_example"
	hashRing := ciredis.NewRedisHashRing(hashRingKey, redisClient)

	cleanup := func() {
		ctx := context.Background()
		_ = redisClient.Del(ctx, "redis:consistent_hash:ring:"+hashRingKey)
		_ = redisClient.Del(ctx, "redis:consistent_hash:ring:node:replica:"+hashRingKey)
		_ = redisClient.Del(ctx, "redis:consistent_hash:ring:lock:"+hashRingKey)
		for _, nodeID := range []string{"node_a", "node_b", "node_c"} {
			_ = redisClient.Del(ctx, "redis:consistent_hash:ring:node:data:"+nodeID)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	consistentHash := consistenthash.NewConsistentHash(hashRing, consistenthash.NewFnvHasher(), nil)
	exerciseConsistentHash(t, consistentHash)
}

// TestLocalLock verifies the expiry based auto release of the local ring lock.
func TestLocalLock(t *testing.T) {
	hashRing := local.NewSkiplistHashRing()
	ctx := context.Background()

	if err := hashRing.Lock(ctx, 1); err != nil {
		t.Fatalf("lock: %v", err)
	}
	time.Sleep(2 * time.Second)
	if err := hashRing.Lock(ctx, 2); err != nil {
		t.Fatalf("lock after expiry: %v", err)
	}
	if err := hashRing.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}

// exerciseConsistentHash walks a small lifecycle: two nodes join, four data
// keys are placed, a third node joins, then leaves again.
func exerciseConsistentHash(t *testing.T, consistentHash *consistenthash.ConsistentHash) {
	t.Helper()
	ctx := context.Background()

	nodes := []struct {
		id     string
		weight int
	}{
		{id: "node_a", weight: 2},
		{id: "node_b", weight: 1},
		{id: "node_c", weight: 1},
	}
	dataKeys := []string{"data_a", "data_b", "data_c", "data_d"}

	for _, node := range nodes[:2] {
		if err := consistentHash.AddNode(ctx, node.id, node.weight); err != nil {
			t.Fatalf("add %s: %v", node.id, err)
		}
	}
	logOwners(t, consistentHash, dataKeys)

	if err := consistentHash.AddNode(ctx, nodes[2].id, nodes[2].weight); err != nil {
		t.Fatalf("add %s: %v", nodes[2].id, err)
	}
	logOwners(t, consistentHash, dataKeys)

	if err := consistentHash.RemoveNode(ctx, nodes[2].id); err != nil {
		t.Fatalf("remove %s: %v", nodes[2].id, err)
	}
	logOwners(t, consistentHash, dataKeys)
}

func logOwners(t *testing.T, consistentHash *consistenthash.ConsistentHash, dataKeys []string) {
	t.Helper()
	ctx := context.Background()
	for _, dataKey := range dataKeys {
		node, err := consistentHash.GetNode(ctx, dataKey)
		if err != nil {
			t.Fatalf("get node for %s: %v", dataKey, err)
		}
		t.Logf("data %s belongs to node %s", dataKey, node)
	}
}
