package bloom

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/hash"
	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/redis"
)

func newTestFilter(t *testing.T) (*Filter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.GetClient(config.NewRedisConfigProvider(&config.RedisConfig{Address: mr.Addr()}))
	return NewFilter(client, hash.NewSHA1Encryptor(), hash.NewMurmur3Encryptor()), mr
}

func TestExistReportsSetValues(t *testing.T) {
	f, _ := newTestFilter(t)
	ctx := context.Background()

	if exist, err := f.Exist(ctx, "bloom", "value-1"); err != nil || exist {
		t.Fatalf("Exist on empty filter = (%t, %v), want (false, nil)", exist, err)
	}

	if err := f.Set(ctx, "bloom", "value-1", 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if exist, err := f.Exist(ctx, "bloom", "value-1"); err != nil || !exist {
		t.Fatalf("Exist after Set = (%t, %v), want (true, nil)", exist, err)
	}
	if exist, err := f.Exist(ctx, "bloom", "value-2"); err != nil || exist {
		t.Fatalf("Exist of an unset value = (%t, %v), want (false, nil)", exist, err)
	}
}

func TestSetAppliesExpiry(t *testing.T) {
	f, mr := newTestFilter(t)
	ctx := context.Background()

	if err := f.Set(ctx, "bloom", "value-1", 60); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !mr.Exists("bloom") {
		t.Fatal("bloom key not stored")
	}
	if ttl := mr.TTL("bloom"); ttl <= 0 || ttl > 60*time.Second {
		t.Fatalf("TTL = %v, want within (0, 60s]", ttl)
	}
}
