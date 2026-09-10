package local

import (
	"context"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"go.uber.org/goleak"
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

// TestSkiplistHashRing_ConcurrentLockDoesNotBlockRelease is the regression
// test for the lock-ordering deadlock: Lock used to take doubleLock before the
// ring mutex, so a goroutine waiting for a held lock blocked the holder's
// Unlock forever. Ownership is tracked per goroutine, so the holder unlocks
// from its own goroutine and the waiter must be able to acquire the lock.
func TestSkiplistHashRing_ConcurrentLockDoesNotBlockRelease(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	holderDone := make(chan error, 1)
	go func() {
		if err := ring.Lock(ctx, 0); err != nil {
			holderDone <- err
			return
		}
		// Give the waiter time to block on the ring mutex before
		// releasing it from this (the owning) goroutine.
		time.Sleep(50 * time.Millisecond)
		holderDone <- ring.Unlock(ctx)
	}()

	waiterDone := make(chan error, 1)
	go func() {
		time.Sleep(10 * time.Millisecond) // let the holder lock first
		if err := ring.Lock(ctx, 0); err != nil {
			waiterDone <- err
			return
		}
		waiterDone <- ring.Unlock(ctx)
	}()

	// The waiter can only acquire the lock after the holder's Unlock
	// returned; a regression would leave both blocked forever.
	select {
	case err := <-waiterDone:
		if err != nil {
			t.Fatalf("waiter: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter never acquired the lock: Lock/Unlock deadlock")
	}
	if err := <-holderDone; err != nil {
		t.Fatalf("holder: %v", err)
	}
}

// TestSkiplistHashRing_NodeReturnsCopy verifies that mutating the slice
// returned by Node cannot corrupt the internal ring state.
func TestSkiplistHashRing_NodeReturnsCopy(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	if err := ring.Add(ctx, 100, "a_0"); err != nil {
		t.Fatalf("add: %v", err)
	}

	ids, err := ring.Node(ctx, 100)
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	ids[0] = "mutated"

	ids, err = ring.Node(ctx, 100)
	if err != nil {
		t.Fatalf("node: %v", err)
	}
	if len(ids) != 1 || ids[0] != "a_0" {
		t.Fatalf("Node must return a copy, got %v", ids)
	}
}

// TestSkiplistHashRing_RandomOperationsAgainstModel drives the skiplist with
// pseudo random add and rem operations and checks every step against a plain
// reference map, covering shared scores, wrap-around lookups and full ring
// sweeps.
func TestSkiplistHashRing_RandomOperationsAgainstModel(t *testing.T) {
	ring := NewSkiplistHashRing()
	ctx := context.Background()

	const (
		scoreMax    = int32(100) // score domain [0, scoreMax)
		ops         = 2000
		sweepPeriod = 200
	)
	ids := []string{"n_0", "n_1", "n_2", "n_3", "n_4"}

	// entries mirrors the expected ring content: score -> node keys in
	// registration order.
	entries := make(map[int32][]string)

	modelCeiling := func(score int32) int32 {
		if len(entries) == 0 {
			return -1
		}
		best := int32(-1)
		for s := range entries {
			if s >= score && (best == -1 || s < best) {
				best = s
			}
		}
		if best != -1 {
			return best
		}
		// Wrap around to the smallest score of the ring.
		best = int32(math.MaxInt32)
		for s := range entries {
			if s < best {
				best = s
			}
		}
		return best
	}
	modelFloor := func(score int32) int32 {
		if len(entries) == 0 {
			return -1
		}
		best := int32(-1)
		for s := range entries {
			if s <= score && s > best {
				best = s
			}
		}
		if best != -1 {
			return best
		}
		// Wrap around to the largest score of the ring.
		for s := range entries {
			if s > best {
				best = s
			}
		}
		return best
	}

	checkLookups := func(probes []int32) {
		t.Helper()
		for _, probe := range probes {
			got, err := ring.Ceiling(ctx, probe)
			assertScore(t, "Ceiling", got, err, modelCeiling(probe))
			got, err = ring.Floor(ctx, probe)
			assertScore(t, "Floor", got, err, modelFloor(probe))
		}
	}

	// sweepRing walks every distinct score via Ceiling and compares the set
	// with the model.
	sweepRing := func() {
		t.Helper()
		if len(entries) == 0 {
			return
		}
		first, err := ring.Ceiling(ctx, 0)
		if err != nil {
			t.Fatalf("sweep ceiling: %v", err)
		}
		scores := []int32{first}
		for {
			next, err := ring.Ceiling(ctx, scores[len(scores)-1]+1)
			if err != nil {
				t.Fatalf("sweep ceiling: %v", err)
			}
			if next == first {
				break
			}
			scores = append(scores, next)
		}
		if len(scores) != len(entries) {
			t.Fatalf("sweep found %d scores, model has %d: %v", len(scores), len(entries), scores)
		}
		for _, score := range scores {
			if _, ok := entries[score]; !ok {
				t.Fatalf("sweep found unexpected score %d", score)
			}
		}
	}

	rng := rand.New(rand.NewPCG(42, 2024))
	for i := range ops {
		var (
			score     int32
			id        string
			expectErr bool
		)

		if rng.IntN(100) < 55 || len(entries) == 0 {
			// Add: a duplicate key under an existing score is idempotent.
			score = int32(rng.IntN(int(scoreMax)))
			id = ids[rng.IntN(len(ids))]

			if err := ring.Add(ctx, score, id); err != nil {
				t.Fatalf("op %d: add (%d, %s): %v", i, score, id, err)
			}
			if !slices.Contains(entries[score], id) {
				entries[score] = append(entries[score], id)
			}
		} else {
			// Rem: always target an existing score.
			existing := make([]int32, 0, len(entries))
			for s := range entries {
				existing = append(existing, s)
			}
			score = existing[rng.IntN(len(existing))]

			if rng.IntN(100) < 20 {
				// Rem of a key that is not registered under the score
				// must fail with an error.
				var ghosts []string
				for _, candidate := range ids {
					if !slices.Contains(entries[score], candidate) {
						ghosts = append(ghosts, candidate)
					}
				}
				if len(ghosts) > 0 {
					id = ghosts[rng.IntN(len(ghosts))]
					if err := ring.Rem(ctx, score, id); err == nil {
						t.Fatalf("op %d: rem of unregistered key (%d, %s) should fail", i, score, id)
					}
					expectErr = true
				}
			}

			if !expectErr {
				// All ids may already be registered under the score, in
				// which case the ghost branch above was skipped and a
				// registered key is removed instead.
				if id == "" {
					id = entries[score][rng.IntN(len(entries[score]))]
				}
				if err := ring.Rem(ctx, score, id); err != nil {
					t.Fatalf("op %d: rem (%d, %s): %v", i, score, id, err)
				}
				remaining := slices.DeleteFunc(slices.Clone(entries[score]), func(k string) bool { return k == id })
				if len(remaining) == 0 {
					delete(entries, score)
				} else {
					entries[score] = remaining
				}
			}
		}

		// Verify the touched score and the wrap-around edges after every op.
		got, nodeErr := ring.Node(ctx, score)
		if want, ok := entries[score]; ok {
			if nodeErr != nil {
				t.Fatalf("op %d: node %d: %v", i, score, nodeErr)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("op %d: node %d: got %v, want %v", i, score, got, want)
			}
		} else if nodeErr == nil {
			t.Fatalf("op %d: node %d should fail, got %v", i, score, got)
		}
		checkLookups([]int32{0, 1, scoreMax / 2, scoreMax - 1, score - 1, score, score + 1})

		if i%sweepPeriod == sweepPeriod-1 {
			sweepRing()
		}
	}
	sweepRing()
}

// TestSkiplistHashRing_GuardianGoroutineCleanup proves with goleak that the
// guardian goroutine of an expiring ring lock exits both after an explicit
// Unlock and after the expiry based auto release.
func TestSkiplistHashRing_GuardianGoroutineCleanup(t *testing.T) {
	defer goleak.VerifyNone(t)

	ring := NewSkiplistHashRing()
	ctx := context.Background()

	// The explicit unlock cancels the guardian.
	if err := ring.Lock(ctx, 1); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := ring.Unlock(ctx); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	// A lock left to expire releases itself and stops its guardian. The
	// relock below blocks until the guardian released the expired lock.
	if err := ring.Lock(ctx, 1); err != nil {
		t.Fatalf("lock: %v", err)
	}
	cycled := make(chan error, 1)
	go func() {
		if err := ring.Lock(ctx, 0); err != nil {
			cycled <- err
			return
		}
		cycled <- ring.Unlock(ctx)
	}()
	select {
	case err := <-cycled:
		if err != nil {
			t.Fatalf("relock after expiry: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock was not released by the guardian after expiry")
	}
}
