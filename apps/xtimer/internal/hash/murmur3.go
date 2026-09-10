package hash

import (
	"github.com/twmb/murmur3"
)

// Sum32 computes the MurmurHash3 x86 32-bit hash of data with the zero seed.
func Sum32(data []byte) uint32 {
	return murmur3.Sum32(data)
}

// Murmur3Encryptor hashes a string into a uint64 using the MurmurHash3 32-bit algorithm.
type Murmur3Encryptor struct{}

func NewMurmur3Encryptor() *Murmur3Encryptor {
	return &Murmur3Encryptor{}
}

func (m *Murmur3Encryptor) Encrypt(origin string) uint64 {
	return uint64(Sum32([]byte(origin)))
}
