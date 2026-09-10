package hash

import (
	"encoding/binary"
	"math/bits"
)

// Murmur3 x86 32-bit constants.
const (
	murmurC1 uint32 = 0xcc9e2d51
	murmurC2 uint32 = 0x1b873593
	// Seed used by Sum32; the previous dependency used the zero seed.
	murmurSeed uint32 = 0
)

// Sum32 computes the MurmurHash3 x86 32-bit hash of data with the zero seed.
func Sum32(data []byte) uint32 {
	h := murmurSeed
	n := len(data) / 4

	for i := 0; i < n; i++ {
		k := binary.LittleEndian.Uint32(data[i*4:])
		k *= murmurC1
		k = bits.RotateLeft32(k, 15)
		k *= murmurC2

		h ^= k
		h = bits.RotateLeft32(h, 13)
		h = h*5 + 0xe6546b64
	}

	var k uint32
	tail := data[n*4:]
	switch len(tail) {
	case 3:
		k ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k ^= uint32(tail[0])
		k *= murmurC1
		k = bits.RotateLeft32(k, 15)
		k *= murmurC2
		h ^= k
	}

	h ^= uint32(len(data))
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// Murmur3Encryptor hashes a string into a uint64 using the MurmurHash3 32-bit algorithm.
type Murmur3Encryptor struct{}

func NewMurmur3Encryptor() *Murmur3Encryptor {
	return &Murmur3Encryptor{}
}

func (m *Murmur3Encryptor) Encrypt(origin string) uint64 {
	return uint64(Sum32([]byte(origin)))
}
