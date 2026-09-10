package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/alicebob/miniredis/v2"

	"github.com/hangtiancheng/swifty.go/apps/consistent_cache"
)

// TestCacheDisableKey verifies the disable key mapping.
func TestCacheDisableKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "regular key",
			key:  "user:1",
			want: "Enable_Lock_Key_{user:1}",
		},
		{
			name: "empty key",
			key:  "",
			want: "Enable_Lock_Key_{}",
		},
	}

	c := &Cache{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.disableKey(tt.key); got != tt.want {
				t.Errorf("disableKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// TestDurationConversions verifies the duration helpers.
func TestDurationConversions(t *testing.T) {
	if got, want := secondsToDuration(30), 30*time.Second; got != want {
		t.Errorf("secondsToDuration(30) = %v, want %v", got, want)
	}
	if got, want := millisToDuration(1500), 1500*time.Millisecond; got != want {
		t.Errorf("millisToDuration(1500) = %v, want %v", got, want)
	}
}

var errFakeClient = errors.New("fake client error")

// fakeClient is a Client implementation that scripts results and records calls.
type fakeClient struct {
	getValue  string
	getErr    error
	evalReply any
	evalErr   error

	// recorded calls.
	evalSrc         string
	evalKeyCount    int
	evalKeysAndArgs []any
	setExCalls      []setExCall
	delKeys         []string
	pExpireCalls    []pExpireCall
}

type setExCall struct {
	key           string
	value         string
	expireSeconds int64
}

type pExpireCall struct {
	key         string
	expireMilis int64
}

func (c *fakeClient) Eval(_ context.Context, src string, keyCount int, keysAndArgs []any) (any, error) {
	c.evalSrc = src
	c.evalKeyCount = keyCount
	c.evalKeysAndArgs = keysAndArgs
	return c.evalReply, c.evalErr
}

func (c *fakeClient) Get(_ context.Context, _ string) (string, error) {
	return c.getValue, c.getErr
}

func (c *fakeClient) SetEx(_ context.Context, key, value string, expireSeconds int64) error {
	c.setExCalls = append(c.setExCalls, setExCall{key: key, value: value, expireSeconds: expireSeconds})
	return nil
}

func (c *fakeClient) Del(_ context.Context, key string) error {
	c.delKeys = append(c.delKeys, key)
	return nil
}

func (c *fakeClient) PExpire(_ context.Context, key string, expireMilis int64) error {
	c.pExpireCalls = append(c.pExpireCalls, pExpireCall{key: key, expireMilis: expireMilis})
	return nil
}

// TestCacheGetMapsRedisNilToCacheMiss verifies the cache miss mapping of Get.
func TestCacheGetMapsRedisNilToCacheMiss(t *testing.T) {
	tests := []struct {
		name      string
		client    *fakeClient
		wantValue string
		wantErr   error
	}{
		{
			name:      "hit returns the value",
			client:    &fakeClient{getValue: "v"},
			wantValue: "v",
		},
		{
			name:    "miss maps redis.Nil to ErrorCacheMiss",
			client:  &fakeClient{getErr: goredis.Nil},
			wantErr: consistent_cache.ErrorCacheMiss,
		},
		{
			name:    "other errors are propagated",
			client:  &fakeClient{getErr: errFakeClient},
			wantErr: errFakeClient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cache{client: tt.client}
			got, err := c.Get(context.Background(), "k")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Get() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.wantValue {
				t.Errorf("Get() = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

// TestCachePutWhenEnable verifies the Lua script invocation and the reply
// handling of PutWhenEnable.
func TestCachePutWhenEnable(t *testing.T) {
	tests := []struct {
		name    string
		client  *fakeClient
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "enabled writes the cache",
			client: &fakeClient{evalReply: int64(1)},
			wantOK: true,
		},
		{
			name:   "disabled skips the write",
			client: &fakeClient{evalReply: int64(0)},
		},
		{
			name:    "unexpected reply type fails",
			client:  &fakeClient{evalReply: "unexpected"},
			wantErr: true,
		},
		{
			name:    "client errors are propagated",
			client:  &fakeClient{evalErr: errFakeClient},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cache{client: tt.client}
			ok, err := c.PutWhenEnable(context.Background(), "k", "v", 42)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PutWhenEnable() error = %v, wantErr %t", err, tt.wantErr)
			}
			if ok != tt.wantOK {
				t.Errorf("PutWhenEnable() = %t, want %t", ok, tt.wantOK)
			}
		})
	}

	// Verify the script invocation arguments of a representative call.
	client := &fakeClient{evalReply: int64(1)}
	c := &Cache{client: client}
	if _, err := c.PutWhenEnable(context.Background(), "k", "v", 42); err != nil {
		t.Fatalf("PutWhenEnable() error = %v", err)
	}
	if client.evalSrc != LuaCheckEnableAndWriteCache {
		t.Errorf("eval src = %q, want the check-enable-and-write script", client.evalSrc)
	}
	if client.evalKeyCount != 2 {
		t.Errorf("eval key count = %d, want 2", client.evalKeyCount)
	}
	wantArgs := []any{"Enable_Lock_Key_{k}", "k", "v", int64(42)}
	if !equalAny(client.evalKeysAndArgs, wantArgs) {
		t.Errorf("eval keys and args = %v, want %v", client.evalKeysAndArgs, wantArgs)
	}
}

// TestCacheEnableAndDisableTargetDisableKey verifies that Enable and Disable
// operate on the disable key of the data key.
func TestCacheEnableAndDisableTargetDisableKey(t *testing.T) {
	client := &fakeClient{}
	c := &Cache{client: client}

	if err := c.Enable(context.Background(), "k", 500); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if got, want := len(client.pExpireCalls), 1; got != want {
		t.Fatalf("pexpire calls = %d, want %d", got, want)
	}
	if got, want := client.pExpireCalls[0], (pExpireCall{key: "Enable_Lock_Key_{k}", expireMilis: 500}); got != want {
		t.Errorf("pexpire call = %+v, want %+v", got, want)
	}

	if err := c.Disable(context.Background(), "k", 7); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if got, want := len(client.setExCalls), 1; got != want {
		t.Fatalf("setex calls = %d, want %d", got, want)
	}
	if got, want := client.setExCalls[0], (setExCall{key: "Enable_Lock_Key_{k}", value: "1", expireSeconds: 7}); got != want {
		t.Errorf("setex call = %+v, want %+v", got, want)
	}

	if err := c.Del(context.Background(), "k"); err != nil {
		t.Fatalf("Del() error = %v", err)
	}
	if got, want := client.delKeys, []string{"k"}; !equalString(got, want) {
		t.Errorf("del keys = %v, want %v", got, want)
	}
}

// TestCacheAgainstMiniredis verifies the cache module against a real Redis
// protocol implementation, including the Lua script of PutWhenEnable.
func TestCacheAgainstMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	c := NewRedisCache(&Config{Address: mr.Addr()})
	ctx := context.Background()

	// A missing key is a cache miss.
	if _, err := c.Get(ctx, "k"); !errors.Is(err, consistent_cache.ErrorCacheMiss) {
		t.Fatalf("Get() error = %v, want %v", err, consistent_cache.ErrorCacheMiss)
	}

	// With the mechanism enabled, the write succeeds and expires.
	ok, err := c.PutWhenEnable(ctx, "k", "v1", 60)
	if err != nil {
		t.Fatalf("PutWhenEnable() error = %v", err)
	}
	if !ok {
		t.Error("PutWhenEnable() = false, want true")
	}
	if got, err := c.Get(ctx, "k"); err != nil || got != "v1" {
		t.Fatalf("Get() = %q, %v, want %q, nil", got, err, "v1")
	}
	if ttl := mr.TTL("k"); ttl <= 0 || ttl > time.Minute {
		t.Errorf("TTL(k) = %v, want within (0, 1m]", ttl)
	}

	// While the write-cache mechanism is disabled, the write is skipped.
	if err := c.Disable(ctx, "k", 60); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	ok, err = c.PutWhenEnable(ctx, "k", "v2", 60)
	if err != nil {
		t.Fatalf("PutWhenEnable() error = %v", err)
	}
	if ok {
		t.Error("PutWhenEnable() = true, want false while disabled")
	}
	if got, err := c.Get(ctx, "k"); err != nil || got != "v1" {
		t.Fatalf("Get() = %q, %v, want the untouched %q, nil", got, err, "v1")
	}

	// Enable shortens the expiry of the disable mark, so the mechanism becomes
	// enabled again after the delay.
	if err := c.Enable(ctx, "k", 100); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	mr.FastForward(200 * time.Millisecond)
	if mr.Exists("Enable_Lock_Key_{k}") {
		t.Fatal("the disable mark is still present after the enable delay")
	}
	ok, err = c.PutWhenEnable(ctx, "k", "v2", 60)
	if err != nil || !ok {
		t.Fatalf("PutWhenEnable() = %t, %v, want true, nil", ok, err)
	}
	if got, err := c.Get(ctx, "k"); err != nil || got != "v2" {
		t.Fatalf("Get() = %q, %v, want %q, nil", got, err, "v2")
	}

	// Del removes the cached value.
	if err := c.Del(ctx, "k"); err != nil {
		t.Fatalf("Del() error = %v", err)
	}
	if _, err := c.Get(ctx, "k"); !errors.Is(err, consistent_cache.ErrorCacheMiss) {
		t.Fatalf("Get() error = %v, want %v", err, consistent_cache.ErrorCacheMiss)
	}
}

func equalAny(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func equalString(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
