package consistent_hash

import "hash/crc32"

// Config controls consistent hash ring behavior.
type Config struct {
	DefaultReplicas      int
	MinReplicas          int
	MaxReplicas          int
	HashFunc             func(data []byte) uint32
	LoadBalanceThreshold float64
}

// DefaultConfig is the default consistent hash configuration.
var DefaultConfig = &Config{
	DefaultReplicas:      50,
	MinReplicas:          10,
	MaxReplicas:          200,
	HashFunc:             crc32.ChecksumIEEE,
	LoadBalanceThreshold: 0.25,
}
