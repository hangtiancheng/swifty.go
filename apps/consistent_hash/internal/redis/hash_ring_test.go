package redis

import (
	"context"
	"net"
	"testing"
	"time"

	redislock "github.com/hangtiancheng/swifty.go/apps/redis_lock"
)

const testRedisAddr = "127.0.0.1:6379"

// requireRedis performs a quick TCP dial so that integration tests skip
// cleanly when no local Redis instance is reachable.
func requireRedis(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", testRedisAddr, 500*time.Millisecond)
	if err != nil {
		t.Skipf("skipping: no redis reachable at %s: %v", testRedisAddr, err)
	}
	_ = conn.Close()
}

// newTestRing builds a ring on a live Redis instance and schedules the cleanup
// of every key it touches.
func newTestRing(t *testing.T, ringKey string, nodeIDs ...string) (*RedisHashRing, *redislock.Client) {
	t.Helper()
	requireRedis(t)

	client := redislock.NewClient("tcp", testRedisAddr, "")
	ring := NewRedisHashRing(ringKey, client)

	cleanupKeys := []string{ring.getTableKey(), ring.getNodeReplicaKey()}
	for _, nodeID := range nodeIDs {
		cleanupKeys = append(cleanupKeys, ring.getNodeDataKey(nodeID))
	}
	cleanup := func() {
		ctx := context.Background()
		for _, key := range cleanupKeys {
			_ = client.Del(ctx, key)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	return ring, client
}

func assertScore(t *testing.T, name string, got int32, err error, want int32) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s: got score %d, want %d", name, got, want)
	}
}

func TestRedisHashRing_AddNodeCeilingFloorRem(t *testing.T) {
	ring, _ := newTestRing(t, "test_ring_order", "a_0", "b_0", "c_0")
	ctx := context.Background()

	// Insert out of order on purpose.
	for _, node := range []struct {
		score int32
		id    string
	}{
		{score: 300, id: "c_0"},
		{score: 100, id: "a_0"},
		{score: 200, id: "b_0"},
	} {
		if err := ring.Add(ctx, node.score, node.id); err != nil {
			t.Fatalf("add %s: %v", node.id, err)
		}
	}

	// Adding the same key under an existing score is idempotent.
	if err := ring.Add(ctx, 100, "a_0"); err != nil {
		t.Fatalf("re-add a_0: %v", err)
	}
	ids, err := ring.Node(ctx, 100)
	if err != nil {
		t.Fatalf("node 100: %v", err)
	}
	if len(ids) != 1 || ids[0] != "a_0" {
		t.Fatalf("node 100: got %v, want [a_0]", ids)
	}

	if ids, err = ring.Node(ctx, 200); err != nil {
		t.Fatalf("node 200: %v", err)
	}
	if len(ids) != 1 || ids[0] != "b_0" {
		t.Fatalf("node 200: got %v, want [b_0]", ids)
	}

	if _, err = ring.Node(ctx, 250); err == nil {
		t.Fatal("Node on missing score should fail")
	}

	got, err := ring.Ceiling(ctx, 50)
	assertScore(t, "Ceiling(50)", got, err, 100)
	got, err = ring.Ceiling(ctx, 100)
	assertScore(t, "Ceiling(100)", got, err, 100)
	got, err = ring.Ceiling(ctx, 150)
	assertScore(t, "Ceiling(150)", got, err, 200)
	got, err = ring.Ceiling(ctx, 300)
	assertScore(t, "Ceiling(300)", got, err, 300)
	got, err = ring.Ceiling(ctx, 301)
	assertScore(t, "Ceiling(301) wraps", got, err, 100)

	got, err = ring.Floor(ctx, 350)
	assertScore(t, "Floor(350)", got, err, 300)
	got, err = ring.Floor(ctx, 300)
	assertScore(t, "Floor(300)", got, err, 300)
	got, err = ring.Floor(ctx, 299)
	assertScore(t, "Floor(299)", got, err, 200)
	got, err = ring.Floor(ctx, 99)
	assertScore(t, "Floor(99) wraps", got, err, 300)

	// Removing a key that is not registered under the score is a no-op, like
	// in the original redis ring.
	if err = ring.Rem(ctx, 200, "ghost"); err != nil {
		t.Fatalf("rem of unregistered key: %v", err)
	}
	if err = ring.Rem(ctx, 200, "b_0"); err != nil {
		t.Fatalf("rem b_0: %v", err)
	}
	if _, err = ring.Node(ctx, 200); err == nil {
		t.Fatal("Node on removed score should fail")
	}

	// The remaining ring still answers lookups correctly.
	got, err = ring.Ceiling(ctx, 150)
	assertScore(t, "Ceiling(150)", got, err, 300)
}

func TestRedisHashRing_EmptyRing(t *testing.T) {
	ring, _ := newTestRing(t, "test_ring_empty")
	ctx := context.Background()

	got, err := ring.Ceiling(ctx, 42)
	assertScore(t, "Ceiling on empty ring", got, err, -1)
	got, err = ring.Floor(ctx, 42)
	assertScore(t, "Floor on empty ring", got, err, -1)
}

func TestRedisHashRing_ReplicaTracking(t *testing.T) {
	ring, _ := newTestRing(t, "test_ring_replicas")
	ctx := context.Background()

	if err := ring.AddNodeToReplica(ctx, "a", 3); err != nil {
		t.Fatalf("add node to replica: %v", err)
	}
	if err := ring.AddNodeToReplica(ctx, "b", 1); err != nil {
		t.Fatalf("add node to replica: %v", err)
	}

	nodes, err := ring.Nodes(ctx)
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	if len(nodes) != 2 || nodes["a"] != 3 || nodes["b"] != 1 {
		t.Fatalf("nodes: got %v", nodes)
	}

	if err = ring.DeleteNodeToReplica(ctx, "a"); err != nil {
		t.Fatalf("delete node to replica: %v", err)
	}
	nodes, err = ring.Nodes(ctx)
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	if len(nodes) != 1 || nodes["b"] != 1 {
		t.Fatalf("nodes after delete: got %v", nodes)
	}
}

func TestRedisHashRing_DataKeys(t *testing.T) {
	ring, _ := newTestRing(t, "test_ring_datakeys", "a", "ghost")
	ctx := context.Background()

	setOf := func(keys ...string) map[string]struct{} {
		out := make(map[string]struct{}, len(keys))
		for _, key := range keys {
			out[key] = struct{}{}
		}
		return out
	}

	// Unknown node yields an empty set, not an error.
	keys, err := ring.DataKeys(ctx, "a")
	if err != nil {
		t.Fatalf("data keys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("data keys of unknown node: got %v", keys)
	}

	if err = ring.AddNodeToDataKeys(ctx, "a", setOf("k1", "k2")); err != nil {
		t.Fatalf("add data keys: %v", err)
	}
	if err = ring.AddNodeToDataKeys(ctx, "a", setOf("k2", "k3")); err != nil {
		t.Fatalf("add data keys: %v", err)
	}

	keys, err = ring.DataKeys(ctx, "a")
	if err != nil {
		t.Fatalf("data keys: %v", err)
	}
	if !equalSets(keys, setOf("k1", "k2", "k3")) {
		t.Fatalf("data keys: got %v, want k1 k2 k3", keys)
	}

	if err = ring.DeleteNodeToDataKeys(ctx, "a", setOf("k1")); err != nil {
		t.Fatalf("delete data keys: %v", err)
	}
	keys, err = ring.DataKeys(ctx, "a")
	if err != nil {
		t.Fatalf("data keys: %v", err)
	}
	if !equalSets(keys, setOf("k2", "k3")) {
		t.Fatalf("data keys after delete: got %v", keys)
	}

	// Deleting the last key drops the stored entry.
	if err = ring.DeleteNodeToDataKeys(ctx, "a", setOf("k2", "k3")); err != nil {
		t.Fatalf("delete data keys: %v", err)
	}
	keys, err = ring.DataKeys(ctx, "a")
	if err != nil {
		t.Fatalf("data keys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("data keys after full delete: got %v", keys)
	}

	// Deleting keys of an untracked node is a no-op.
	if err = ring.DeleteNodeToDataKeys(ctx, "ghost", setOf("k1")); err != nil {
		t.Fatalf("delete data keys of unknown node: %v", err)
	}
}

func TestRedisHashRing_LockUnlock(t *testing.T) {
	ring, _ := newTestRing(t, "test_ring_lock")
	ctx := context.Background()

	if err := ring.Lock(ctx, 10); err != nil {
		t.Fatalf("lock: %v", err)
	}

	// A second non-blocking acquire fails while the lock is held.
	if err := ring.Lock(ctx, 10); err == nil {
		t.Fatal("second lock while held should fail")
	}

	if err := ring.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	// The lock can be acquired again after the release.
	if err := ring.Lock(ctx, 10); err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	if err := ring.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}

func equalSets(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if _, ok := b[key]; !ok {
			return false
		}
	}
	return true
}
