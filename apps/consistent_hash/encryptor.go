package consistent_hash

import (
	"hash/fnv"
	"math"
)

// Encryptor maps an arbitrary string (a data key or a virtual node key) onto
// its position on the hash ring.
type Encryptor interface {
	Encrypt(origin string) int32
}

// FnvHasher is an Encryptor backed by the 64-bit FNV-1a hash function.
type FnvHasher struct{}

// NewFnvHasher returns an Encryptor based on FNV-1a.
func NewFnvHasher() *FnvHasher {
	return &FnvHasher{}
}

// Encrypt hashes origin with FNV-1a, avalanches the raw hash state and folds
// the result into the [0, math.MaxInt32) score range used by the hash ring.
//
// The avalanche step is essential: plain FNV-1a maps inputs that differ only
// in their trailing bytes (e.g. the virtual node keys "node_a_0".."node_a_9")
// to nearby values, which would clump all virtual nodes of one physical node
// into a small segment of the ring.
func (f *FnvHasher) Encrypt(origin string) int32 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(origin))

	h := hasher.Sum64()
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33

	return int32(h % math.MaxInt32)
}
