package swifty_cache

import "hash/crc32"

// Config controls consistent hash ring behavior.
type ConHashConfig struct {
	DefaultReplicas      int
	MinReplicas          int
	MaxReplicas          int
	HashFunc             func(data []byte) uint32
	LoadBalanceThreshold float64
}

// DefaultConHashConfig is the default consistent hash configuration.
var DefaultConHashConfig = &ConHashConfig{
	DefaultReplicas:      50,
	MinReplicas:          10,
	MaxReplicas:          200,
	HashFunc:             crc32.ChecksumIEEE,
	LoadBalanceThreshold: 0.25,
}
