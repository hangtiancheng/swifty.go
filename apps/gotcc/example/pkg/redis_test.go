package pkg

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestBuildKeys(t *testing.T) {
	if got := BuildTXKey("component", "tx"); got != "txKey:component:tx" {
		t.Fatalf("BuildTXKey() = %q", got)
	}
	if got := BuildTXDetailKey("component", "tx"); got != "txDetailKey:component:tx" {
		t.Fatalf("BuildTXDetailKey() = %q", got)
	}
	if got := BuildDataKey("component", "tx", "biz"); got != "txKey:component:tx:biz" {
		t.Fatalf("BuildDataKey() = %q", got)
	}
	if got := BuildTXLockKey("component", "tx"); got != "txLockKey:component:tx" {
		t.Fatalf("BuildTXLockKey() = %q", got)
	}
	if got := BuildTXRecordLockKey(); got != "gotcc:txRecord:lock" {
		t.Fatalf("BuildTXRecordLockKey() = %q", got)
	}
}

func TestNewRedisClient(t *testing.T) {
	client := NewRedisClient("tcp", "127.0.0.1:6379", "")
	if client == nil {
		t.Fatal("NewRedisClient() must not return nil")
	}
}

func TestRedisClientRoundTripLive(t *testing.T) {
	network := envOr("GOTCC_TEST_REDIS_NETWORK", "tcp")
	addr := envOr("GOTCC_TEST_REDIS_ADDR", "127.0.0.1:6379")
	password := os.Getenv("GOTCC_TEST_REDIS_PASSWORD")

	client := NewRedisClient(network, addr, password)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Skipf("live Redis not reachable at %s: %v", addr, err)
	}

	key := "gotcc_test_roundtrip"
	defer func() {
		_ = client.Del(context.Background(), key)
	}()

	if _, err := client.Set(ctx, key, "v1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := client.Get(ctx, key)
	if err != nil || got != "v1" {
		t.Fatalf("Get() = %q, %v; want \"v1\", nil", got, err)
	}

	reply, err := client.SetNX(ctx, key, "v2")
	if err != nil || reply != 0 {
		t.Fatalf("SetNX() on existing key = %d, %v; want 0, nil", reply, err)
	}
	reply, err = client.SetNX(ctx, key+"-nx", "v2")
	if err != nil || reply != 1 {
		t.Fatalf("SetNX() on missing key = %d, %v; want 1, nil", reply, err)
	}
	defer func() {
		_ = client.Del(context.Background(), key+"-nx")
	}()

	if err := client.Del(ctx, key); err != nil {
		t.Fatalf("Del() error = %v", err)
	}
	if _, err := client.Get(ctx, key); err == nil {
		t.Fatal("Get() must fail with a nil error sentinel after Del")
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
