package consistent_hash_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	consistenthash "github.com/hangtiancheng/swifty.go/apps/consistent_hash"
	"github.com/hangtiancheng/swifty.go/apps/consistent_hash/internal/local"
)

// stubEncryptor returns a pre-computed score per key, which keeps the
// migration planning tests fully deterministic.
type stubEncryptor struct {
	scores map[string]int32
}

func (s stubEncryptor) Encrypt(origin string) int32 {
	return s.scores[origin]
}

// migrationRecord captures one migrator invocation.
type migrationRecord struct {
	from     string
	to       string
	dataKeys map[string]struct{}
}

// migrationRecorder is a thread-safe Migrator that records every invocation.
type migrationRecorder struct {
	mu      sync.Mutex
	records []migrationRecord
}

func (m *migrationRecorder) migrator(_ context.Context, dataKeys map[string]struct{}, from, to string) error {
	keys := make(map[string]struct{}, len(dataKeys))
	for dataKey := range dataKeys {
		keys[dataKey] = struct{}{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, migrationRecord{from: from, to: to, dataKeys: keys})
	return nil
}

func (m *migrationRecorder) all() []migrationRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]migrationRecord, len(m.records))
	copy(out, m.records)
	return out
}

func setOf(keys ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
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

// TestConsistentHash_AddNodeMigratesKeyFromSuccessorOwner verifies that adding
// a node between two existing nodes moves the keys of the successor owner that
// fall into the new range, and that removing the node moves them back.
func TestConsistentHash_AddNodeMigratesKeyFromSuccessorOwner(t *testing.T) {
	encryptor := stubEncryptor{scores: map[string]int32{
		"nodeA_0": 100,
		"nodeB_0": 200,
		"nodeC_0": 120,
		"dataA":   110, // owned by nodeB on {100, 200}, by nodeC on {100, 120, 200}
	}}
	recorder := &migrationRecorder{}
	ch := consistenthash.NewConsistentHash(
		local.NewSkiplistHashRing(),
		encryptor,
		recorder.migrator,
		consistenthash.WithReplicas(1),
	)

	ctx := context.Background()
	if err := ch.AddNode(ctx, "nodeA", 1); err != nil {
		t.Fatalf("add nodeA: %v", err)
	}
	if err := ch.AddNode(ctx, "nodeB", 1); err != nil {
		t.Fatalf("add nodeB: %v", err)
	}

	node, err := ch.GetNode(ctx, "dataA")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "nodeB" {
		t.Fatalf("dataA owner: got %s, want nodeB", node)
	}

	if err = ch.AddNode(ctx, "nodeC", 1); err != nil {
		t.Fatalf("add nodeC: %v", err)
	}

	records := recorder.all()
	if len(records) != 1 {
		t.Fatalf("migrations after adding nodeC: got %d records, want 1: %+v", len(records), records)
	}
	if records[0].from != "nodeB" || records[0].to != "nodeC" || !equalSets(records[0].dataKeys, setOf("dataA")) {
		t.Fatalf("unexpected migration record: %+v", records[0])
	}

	if node, err = ch.GetNode(ctx, "dataA"); err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "nodeC" {
		t.Fatalf("dataA owner after adding nodeC: got %s, want nodeC", node)
	}

	if err = ch.RemoveNode(ctx, "nodeC"); err != nil {
		t.Fatalf("remove nodeC: %v", err)
	}

	records = recorder.all()
	if len(records) != 2 {
		t.Fatalf("migrations after removing nodeC: got %d records, want 2: %+v", len(records), records)
	}
	if records[1].from != "nodeC" || records[1].to != "nodeB" || !equalSets(records[1].dataKeys, setOf("dataA")) {
		t.Fatalf("unexpected migration record: %+v", records[1])
	}

	if node, err = ch.GetNode(ctx, "dataA"); err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "nodeB" {
		t.Fatalf("dataA owner after removing nodeC: got %s, want nodeB", node)
	}
}

// TestConsistentHash_AddNodeWithSmallestScoreMigratesWrappedKeys verifies
// migration planning when the new node owns the wrapped segment that spans the
// ring end (the predecessor score is numerically greater than the new one).
func TestConsistentHash_AddNodeWithSmallestScoreMigratesWrappedKeys(t *testing.T) {
	encryptor := stubEncryptor{scores: map[string]int32{
		"nodeA_0": 200,
		"nodeB_0": 300,
		"nodeC_0": 100, // smallest score, owns [0, 100] plus the wrapped tail
		"dataA":   50,  // owned by nodeA on {200, 300}, by nodeC once 100 exists
	}}
	recorder := &migrationRecorder{}
	ch := consistenthash.NewConsistentHash(
		local.NewSkiplistHashRing(),
		encryptor,
		recorder.migrator,
		consistenthash.WithReplicas(1),
	)

	ctx := context.Background()
	if err := ch.AddNode(ctx, "nodeA", 1); err != nil {
		t.Fatalf("add nodeA: %v", err)
	}
	if err := ch.AddNode(ctx, "nodeB", 1); err != nil {
		t.Fatalf("add nodeB: %v", err)
	}

	node, err := ch.GetNode(ctx, "dataA")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "nodeA" {
		t.Fatalf("dataA owner: got %s, want nodeA", node)
	}

	if err = ch.AddNode(ctx, "nodeC", 1); err != nil {
		t.Fatalf("add nodeC: %v", err)
	}

	records := recorder.all()
	if len(records) != 1 {
		t.Fatalf("migrations after adding nodeC: got %d records, want 1: %+v", len(records), records)
	}
	if records[0].from != "nodeA" || records[0].to != "nodeC" || !equalSets(records[0].dataKeys, setOf("dataA")) {
		t.Fatalf("unexpected migration record: %+v", records[0])
	}

	if node, err = ch.GetNode(ctx, "dataA"); err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "nodeC" {
		t.Fatalf("dataA owner after adding nodeC: got %s, want nodeC", node)
	}
}

// TestConsistentHash_ErrorPaths covers duplicate adds, removals of unknown
// nodes and lookups on an empty ring.
func TestConsistentHash_ErrorPaths(t *testing.T) {
	encryptor := stubEncryptor{scores: map[string]int32{"nodeA_0": 100}}
	ch := consistenthash.NewConsistentHash(
		local.NewSkiplistHashRing(),
		encryptor,
		nil,
		consistenthash.WithReplicas(1),
	)

	ctx := context.Background()
	if _, err := ch.GetNode(ctx, "dataA"); err == nil {
		t.Fatal("GetNode on empty ring should fail")
	} else if !strings.Contains(err.Error(), "no node available") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := ch.AddNode(ctx, "nodeA", 1); err != nil {
		t.Fatalf("add nodeA: %v", err)
	}
	if err := ch.AddNode(ctx, "nodeA", 1); err == nil {
		t.Fatal("duplicate AddNode should fail")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := ch.RemoveNode(ctx, "ghost"); err == nil {
		t.Fatal("RemoveNode of unknown node should fail")
	} else if !strings.Contains(err.Error(), "invalid node id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestConsistentHash_WeightedVirtualNodes verifies that weight * replicas
// virtual nodes are created and removed together with the node.
func TestConsistentHash_WeightedVirtualNodes(t *testing.T) {
	encryptor := stubEncryptor{scores: map[string]int32{
		"nodeA_0": 10,
		"nodeA_1": 20,
		"nodeA_2": 30,
		"nodeA_3": 40,
	}}
	ring := local.NewSkiplistHashRing()
	ch := consistenthash.NewConsistentHash(ring, encryptor, nil, consistenthash.WithReplicas(2))

	ctx := context.Background()
	if err := ch.AddNode(ctx, "nodeA", 2); err != nil {
		t.Fatalf("add nodeA: %v", err)
	}

	nodes, err := ring.Nodes(ctx)
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	if len(nodes) != 1 || nodes["nodeA"] != 4 {
		t.Fatalf("virtual node count: got %v, want {nodeA: 4}", nodes)
	}

	node, err := ch.GetNode(ctx, "dataA")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "nodeA" {
		t.Fatalf("dataA owner: got %s, want nodeA", node)
	}

	if err = ch.RemoveNode(ctx, "nodeA"); err != nil {
		t.Fatalf("remove nodeA: %v", err)
	}
	nodes, err = ring.Nodes(ctx)
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("nodes after removal: got %v, want empty", nodes)
	}
}

// TestConsistentHash_GetNodeStripsVirtualSuffix pins the contract that GetNode
// returns the node id, not the raw virtual node key.
func TestConsistentHash_GetNodeStripsVirtualSuffix(t *testing.T) {
	encryptor := stubEncryptor{scores: map[string]int32{
		"node_a_0": 100,
		"data_a":   150,
	}}
	ch := consistenthash.NewConsistentHash(
		local.NewSkiplistHashRing(),
		encryptor,
		nil,
		consistenthash.WithReplicas(1),
	)

	ctx := context.Background()
	if err := ch.AddNode(ctx, "node_a", 1); err != nil {
		t.Fatalf("add node_a: %v", err)
	}

	node, err := ch.GetNode(ctx, "data_a")
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if node != "node_a" {
		t.Fatalf("GetNode returned the virtual node key %q instead of the node id", node)
	}
}
