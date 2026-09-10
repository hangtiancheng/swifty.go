package redis

import (
	"context"
	"testing"

	consistenthash "github.com/hangtiancheng/swifty.go/apps/consistent_hash"
	redislock "github.com/hangtiancheng/swifty.go/apps/redis_lock"
)

// TestConsistentHashWithRedisRing runs the full consistent hash flow on top of
// the Redis backed ring: nodes join, data keys are placed and migrate when a
// node leaves. It requires a local Redis and skips otherwise.
func TestConsistentHashWithRedisRing(t *testing.T) {
	requireRedis(t)

	const ringKey = "test_ring_full_flow"
	client := redislock.NewClient("tcp", testRedisAddr, "")
	ring := NewRedisHashRing(ringKey, client)

	cleanup := func() {
		ctx := context.Background()
		_ = client.Del(ctx, ring.getTableKey())
		_ = client.Del(ctx, ring.getNodeReplicaKey())
		_ = client.Del(ctx, ring.getLockKey())
		for _, nodeID := range []string{"node_a", "node_b", "node_c"} {
			_ = client.Del(ctx, ring.getNodeDataKey(nodeID))
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	loggingMigrator := func(_ context.Context, dataKeys map[string]struct{}, from, to string) error {
		t.Logf("migrating %v from %s to %s", dataKeys, from, to)
		return nil
	}
	consistentHash := consistenthash.NewConsistentHash(
		ring,
		consistenthash.NewFnvHasher(),
		loggingMigrator,
		consistenthash.WithReplicas(2),
	)

	ctx := context.Background()
	if err := consistentHash.AddNode(ctx, "node_a", 2); err != nil {
		t.Fatalf("add node_a: %v", err)
	}
	if err := consistentHash.AddNode(ctx, "node_b", 1); err != nil {
		t.Fatalf("add node_b: %v", err)
	}

	for _, dataKey := range []string{"data_a", "data_b", "data_c", "data_d"} {
		node, err := consistentHash.GetNode(ctx, dataKey)
		if err != nil {
			t.Fatalf("get node for %s: %v", dataKey, err)
		}
		t.Logf("data %s belongs to node %s", dataKey, node)
	}

	if err := consistentHash.AddNode(ctx, "node_c", 1); err != nil {
		t.Fatalf("add node_c: %v", err)
	}
	if err := consistentHash.RemoveNode(ctx, "node_c"); err != nil {
		t.Fatalf("remove node_c: %v", err)
	}

	for _, dataKey := range []string{"data_a", "data_b", "data_c", "data_d"} {
		if _, err := consistentHash.GetNode(ctx, dataKey); err != nil {
			t.Fatalf("get node for %s after removal: %v", dataKey, err)
		}
	}
}
