package consistent_hash_test

import (
	"testing"

	consistenthash "github.com/hangtiancheng/swifty.go/apps/consistent_hash"
)

func TestFnvHasher_Deterministic(t *testing.T) {
	hasher := consistenthash.NewFnvHasher()

	first := hasher.Encrypt("data_a")
	if first != hasher.Encrypt("data_a") {
		t.Fatal("Encrypt must be deterministic")
	}
}

func TestFnvHasher_ScoreRange(t *testing.T) {
	hasher := consistenthash.NewFnvHasher()

	for _, key := range []string{"", "data_a", "node_b_3", "a longer data key with spaces and symbols !@#"} {
		if score := hasher.Encrypt(key); score < 0 {
			t.Fatalf("Encrypt(%q) produced negative score %d", key, score)
		}
	}
}

func TestFnvHasher_DistinctInputs(t *testing.T) {
	hasher := consistenthash.NewFnvHasher()

	seen := make(map[int32]string)
	for _, key := range []string{"data_a", "data_b", "data_c", "node_a_0", "node_b_0"} {
		score := hasher.Encrypt(key)
		if other, ok := seen[score]; ok {
			t.Fatalf("unexpected collision between %q and %q", key, other)
		}
		seen[score] = key
	}
}
