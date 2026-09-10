package local

import (
	"context"
	"testing"
	"time"
)

func assertScore(t *testing.T, name string, got int32, err error, want int32) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s: got score %d, want %d", name, got, want)
	}
}

func TestSkiplistHashRing_OrderingAndWrapping(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	// Insert the virtual nodes out of order on purpose.
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

	// Ceiling walks forward and wraps around the ring end.
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

	// Floor walks backward and wraps around the ring start.
	got, err = ring.Floor(ctx, 350)
	assertScore(t, "Floor(350)", got, err, 300)
	got, err = ring.Floor(ctx, 300)
	assertScore(t, "Floor(300)", got, err, 300)
	got, err = ring.Floor(ctx, 299)
	assertScore(t, "Floor(299)", got, err, 200)
	got, err = ring.Floor(ctx, 100)
	assertScore(t, "Floor(100)", got, err, 100)
	got, err = ring.Floor(ctx, 99)
	assertScore(t, "Floor(99) wraps", got, err, 300)
}

func TestSkiplistHashRing_EmptyRing(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	got, err := ring.Ceiling(ctx, 42)
	assertScore(t, "Ceiling on empty ring", got, err, -1)
	got, err = ring.Floor(ctx, 42)
	assertScore(t, "Floor on empty ring", got, err, -1)

	if _, err = ring.Node(ctx, 42); err == nil {
		t.Fatal("Node on empty ring should fail")
	}
}

func TestSkiplistHashRing_Rem(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	for _, node := range []struct {
		score int32
		id    string
	}{
		{score: 100, id: "a_0"},
		{score: 100, id: "a_1"},
		{score: 200, id: "b_0"},
	} {
		if err := ring.Add(ctx, node.score, node.id); err != nil {
			t.Fatalf("add %s: %v", node.id, err)
		}
	}

	// Removing an unregistered key fails.
	if err := ring.Rem(ctx, 100, "ghost"); err == nil {
		t.Fatal("Rem of unregistered key should fail")
	}

	// Removing one of two keys keeps the virtual node alive.
	if err := ring.Rem(ctx, 100, "a_1"); err != nil {
		t.Fatalf("rem a_1: %v", err)
	}
	ids, err := ring.Node(ctx, 100)
	if err != nil {
		t.Fatalf("node 100: %v", err)
	}
	if len(ids) != 1 || ids[0] != "a_0" {
		t.Fatalf("node 100: got %v, want [a_0]", ids)
	}

	// Removing the last key unlinks the virtual node.
	if err := ring.Rem(ctx, 100, "a_0"); err != nil {
		t.Fatalf("rem a_0: %v", err)
	}
	if _, err = ring.Node(ctx, 100); err == nil {
		t.Fatal("Node on removed score should fail")
	}

	// The remaining ring still answers lookups correctly.
	got, err := ring.Ceiling(ctx, 150)
	assertScore(t, "Ceiling(150)", got, err, 200)
	got, err = ring.Floor(ctx, 150)
	assertScore(t, "Floor(150)", got, err, 200)
}

func TestSkiplistHashRing_ReplicaTracking(t *testing.T) {
	ring := NewSkiplistHashRing()
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

	// The returned map must be a copy, not the internal state.
	nodes["a"] = 99
	nodes, err = ring.Nodes(ctx)
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	if nodes["a"] != 3 {
		t.Fatalf("Nodes must return a copy, got %v", nodes)
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

func TestSkiplistHashRing_DataKeys(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	// Unknown node yields an empty set, not an error.
	keys, err := ring.DataKeys(ctx, "a")
	if err != nil {
		t.Fatalf("data keys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("data keys of unknown node: got %v", keys)
	}

	setOf := func(keys ...string) map[string]struct{} {
		out := make(map[string]struct{}, len(keys))
		for _, key := range keys {
			out[key] = struct{}{}
		}
		return out
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
	if len(keys) != 3 {
		t.Fatalf("data keys: got %v, want k1 k2 k3", keys)
	}

	if err = ring.DeleteNodeToDataKeys(ctx, "a", setOf("k1")); err != nil {
		t.Fatalf("delete data keys: %v", err)
	}
	keys, err = ring.DataKeys(ctx, "a")
	if err != nil {
		t.Fatalf("data keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("data keys after delete: got %v", keys)
	}

	// Deleting the last key drops the node entry.
	if err = ring.DeleteNodeToDataKeys(ctx, "a", setOf("k2", "k3")); err != nil {
		t.Fatalf("delete data keys: %v", err)
	}
	if _, ok := ring.nodeToDataKey["a"]; ok {
		t.Fatal("node entry should be dropped when no key is left")
	}

	// Deleting keys of an untracked node is a no-op.
	if err = ring.DeleteNodeToDataKeys(ctx, "ghost", setOf("k1")); err != nil {
		t.Fatalf("delete data keys of unknown node: %v", err)
	}
}

func TestSkiplistHashRing_Lock(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	if err := ring.Lock(ctx, 1); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := ring.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	// Unlocking twice fails instead of panicking on a double mutex unlock.
	if err := ring.Unlock(ctx); err == nil {
		t.Fatal("second unlock should fail")
	}

	// After the expiry elapsed the guardian releases the lock, so it can be
	// acquired again.
	if err := ring.Lock(ctx, 1); err != nil {
		t.Fatalf("lock: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	if err := ring.Lock(ctx, 0); err != nil {
		t.Fatalf("lock after expiry: %v", err)
	}
	if err := ring.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}
}
