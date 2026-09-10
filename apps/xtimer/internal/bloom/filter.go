package bloom

import (
	"context"
	"math"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/hash"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
)

// m: length of the bit vector. The implementation is backed by Redis; a single
// bitmap can hold at most 512MB, i.e. 2^32 bits, so m = 2^32.
// n: number of elements in the filter. Vectors are isolated per day; assuming
// 1 million executed tasks per day, n = 10^6.
// The false positive probability for k, m, n is (1-e^(-nk/m))^k. With k = 3 the
// probability is 2 * 10^(-10); with k = 2 it is 2 * 10^(-7).
// Therefore k = 2 is sufficient.
// The murmur3 and SHA1 hash functions are used, and each result is taken
// modulo 2^31 for the bit offset.
type Filter struct {
	client     *redis.Client
	encryptor1 *hash.SHA1Encryptor
	encryptor2 *hash.Murmur3Encryptor
}

func NewFilter(client *redis.Client, encryptor1 *hash.SHA1Encryptor, encryptor2 *hash.Murmur3Encryptor) *Filter {
	return &Filter{
		client:     client,
		encryptor1: encryptor1,
		encryptor2: encryptor2,
	}
}

func (f *Filter) Exist(ctx context.Context, key, val string) (bool, error) {
	// Check membership in the bloom filter.
	rawVal1 := f.encryptor1.Encrypt(val)
	if exist, err := f.client.GetBit(ctx, key, int32(rawVal1%math.MaxInt32)); err != nil || exist {
		return exist, err
	}

	rawVal2 := f.encryptor2.Encrypt(val)
	return f.client.GetBit(ctx, key, int32(rawVal2%math.MaxInt32))
}

func (f *Filter) Set(ctx context.Context, key, val string, expireSeconds int64) error {
	// Check whether the key exists; if not, the expiry has to be set.
	// Atomicity is not guaranteed here on purpose.
	existed, _ := f.client.Exists(ctx, key)

	// Compute the offsets of both hash functions and set the bits.
	rawVal1, rawVal2 := f.encryptor1.Encrypt(val), f.encryptor2.Encrypt(val)
	_, err := f.client.Transaction(ctx, redis.NewSetBitCommand(key, int32(rawVal1%math.MaxInt32), 1),
		redis.NewSetBitCommand(key, int32(rawVal2%math.MaxInt32), 1))

	if !existed {
		_ = f.client.Expire(ctx, key, expireSeconds)
	}
	return err
}
