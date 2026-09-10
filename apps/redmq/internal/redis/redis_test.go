package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	network      = "tcp"
	testAddress  = "127.0.0.1:6379"
	testPassword = ""
)

// skipWithoutRedis skips the test when no Redis server is reachable at
// localhost:6379.
func skipWithoutRedis(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout(network, testAddress, 500*time.Millisecond)
	if err != nil {
		t.Skipf("skipping: no redis server reachable at %s: %v", testAddress, err)
	}
	_ = conn.Close()
}

func Test_toString(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "string", in: "value", want: "value"},
		{name: "bytes", in: []byte("value"), want: "value"},
		{name: "int", in: 42, want: "42"},
		{name: "int64", in: int64(-7), want: "-7"},
		{name: "float64", in: 1.5, want: "1.5"},
		{name: "bool", in: true, want: "true"},
		{name: "fallback", in: struct{ A int }{A: 1}, want: "{1}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toString(tt.in); got != tt.want {
				t.Errorf("toString(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func Test_toMsgEntities(t *testing.T) {
	tests := []struct {
		name string
		in   []goredis.XMessage
		want []*MsgEntity
	}{
		{
			name: "nil messages",
			in:   nil,
			want: []*MsgEntity{},
		},
		{
			name: "single field message",
			in: []goredis.XMessage{
				{ID: "1692066364494-0", Values: map[string]any{"first_key": "first_val"}},
			},
			want: []*MsgEntity{
				{MsgID: "1692066364494-0", Key: "first_key", Val: "first_val"},
			},
		},
		{
			name: "multiple messages",
			in: []goredis.XMessage{
				{ID: "1-1", Values: map[string]any{"k1": "v1"}},
				{ID: "1-2", Values: map[string]any{"k2": 100}},
			},
			want: []*MsgEntity{
				{MsgID: "1-1", Key: "k1", Val: "v1"},
				{MsgID: "1-2", Key: "k2", Val: "100"},
			},
		},
		{
			name: "message without fields",
			in: []goredis.XMessage{
				{ID: "1-3", Values: map[string]any{}},
			},
			want: []*MsgEntity{
				{MsgID: "1-3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toMsgEntities(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toMsgEntities() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_repairClient(t *testing.T) {
	tests := []struct {
		name string
		in   ClientOptions
		want ClientOptions
	}{
		{
			name: "negative values fall back to defaults",
			in:   ClientOptions{maxIdle: -1, idleTimeoutSeconds: -1, maxActive: -1},
			want: ClientOptions{maxIdle: DefaultMaxIdle, idleTimeoutSeconds: DefaultIdleTimeoutSeconds, maxActive: DefaultMaxActive},
		},
		{
			name: "valid values are kept",
			in:   ClientOptions{maxIdle: 5, idleTimeoutSeconds: 60, maxActive: 10},
			want: ClientOptions{maxIdle: 5, idleTimeoutSeconds: 60, maxActive: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.in
			repairClient(&opts)
			if !reflect.DeepEqual(opts, tt.want) {
				t.Errorf("repairClient() = %+v, want %+v", opts, tt.want)
			}
		})
	}
}

func Test_xReadGroup_param_validation(t *testing.T) {
	client := NewClient(network, testAddress, testPassword)
	ctx := context.Background()

	tests := []struct {
		name       string
		groupID    string
		consumerID string
		topic      string
	}{
		{name: "empty group id", groupID: "", consumerID: "c", topic: "t"},
		{name: "empty consumer id", groupID: "g", consumerID: "", topic: "t"},
		{name: "empty topic", groupID: "g", consumerID: "c", topic: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := client.xReadGroup(ctx, tt.groupID, tt.consumerID, tt.topic, 100, false); !errors.Is(err, ErrInvalidXReadGroupArgs) {
				t.Errorf("xReadGroup() error = %v, want %v", err, ErrInvalidXReadGroupArgs)
			}
		})
	}
}

func Test_param_validation(t *testing.T) {
	client := NewClient(network, testAddress, testPassword)
	defer client.cli.Close()
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "XADD empty topic",
			call: func() error { _, err := client.XADD(ctx, "", 10, "k", "v"); return err },
		},
		{
			name: "XACK empty topic",
			call: func() error { return client.XACK(ctx, "", "g", "1-1") },
		},
		{
			name: "XACK empty group id",
			call: func() error { return client.XACK(ctx, "t", "", "1-1") },
		},
		{
			name: "XACK empty msg id",
			call: func() error { return client.XACK(ctx, "t", "g", "") },
		},
		{
			name: "GET empty key",
			call: func() error { _, err := client.Get(ctx, ""); return err },
		},
		{
			name: "SET empty key",
			call: func() error { _, err := client.Set(ctx, "", "v"); return err },
		},
		{
			name: "SET empty value",
			call: func() error { _, err := client.Set(ctx, "k", ""); return err },
		},
		{
			name: "SetNEX empty key",
			call: func() error { _, err := client.SetNEX(ctx, "", "v", 10); return err },
		},
		{
			name: "SetNEX empty value",
			call: func() error { _, err := client.SetNEX(ctx, "k", "", 10); return err },
		},
		{
			name: "SetNX empty key",
			call: func() error { _, err := client.SetNX(ctx, "", "v"); return err },
		},
		{
			name: "SetNX empty value",
			call: func() error { _, err := client.SetNX(ctx, "k", ""); return err },
		},
		{
			name: "DEL empty key",
			call: func() error { return client.Del(ctx, "") },
		},
		{
			name: "INCR empty key",
			call: func() error { _, err := client.Incr(ctx, ""); return err },
		},
		{
			name: "EVAL negative key count",
			call: func() error { _, err := client.Eval(ctx, "return 1", -1, nil); return err },
		},
		{
			name: "EVAL too few keys",
			call: func() error { _, err := client.Eval(ctx, "return 1", 2, []any{"k1"}); return err },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Errorf("%s succeeded, want a validation error", tt.name)
			}
		})
	}
}

func Test_xadd_and_xreadgroup(t *testing.T) {
	skipWithoutRedis(t)

	client := NewClient(network, testAddress, testPassword)
	defer client.cli.Close()

	ctx := context.Background()
	topic := fmt.Sprintf("redmq_test_topic_%d", time.Now().UnixNano())
	group := fmt.Sprintf("redmq_test_group_%d", time.Now().UnixNano())

	// XADD creates the topic stream; the group starts from 0-0 so every
	// message already in the stream is delivered to the group.
	msgID, err := client.XADD(ctx, topic, 10, "test_key", "test_val")
	if err != nil {
		t.Fatalf("XADD failed: %v", err)
	}
	if msgID == "" {
		t.Fatal("XADD returned an empty message id")
	}

	if _, err := client.XGroupCreate(ctx, topic, group); err != nil {
		t.Fatalf("XGroupCreate failed: %v", err)
	}

	msgs, err := client.XReadGroup(ctx, group, "test_consumer", topic, 1000)
	if err != nil {
		t.Fatalf("XReadGroup failed: %v", err)
	}
	want := []*MsgEntity{{MsgID: msgID, Key: "test_key", Val: "test_val"}}
	if !reflect.DeepEqual(msgs, want) {
		t.Errorf("XReadGroup() = %+v, want %+v", msgs, want)
	}

	if err := client.XACK(ctx, topic, group, msgID); err != nil {
		t.Fatalf("XACK failed: %v", err)
	}

	// the acknowledged message is no longer part of the pending entries list
	msgs, err = client.XReadGroupPending(ctx, group, "test_consumer", topic)
	if !errors.Is(err, ErrNoMsg) {
		t.Errorf("XReadGroupPending() = %+v, err = %v, want ErrNoMsg", msgs, err)
	}
}

func Test_xreadgroup_pending_before_ack(t *testing.T) {
	skipWithoutRedis(t)

	client := NewClient(network, testAddress, testPassword)
	defer client.cli.Close()

	ctx := context.Background()
	topic := fmt.Sprintf("redmq_test_topic_%d", time.Now().UnixNano())
	group := fmt.Sprintf("redmq_test_group_%d", time.Now().UnixNano())

	msgID, err := client.XADD(ctx, topic, 10, "test_key", "test_val")
	if err != nil {
		t.Fatalf("XADD failed: %v", err)
	}

	if _, err := client.XGroupCreate(ctx, topic, group); err != nil {
		t.Fatalf("XGroupCreate failed: %v", err)
	}

	if _, err := client.XReadGroup(ctx, group, "test_consumer", topic, 1000); err != nil {
		t.Fatalf("XReadGroup failed: %v", err)
	}

	// an unacknowledged message stays in the pending entries list of the
	// consumer it was delivered to
	msgs, err := client.XReadGroupPending(ctx, group, "test_consumer", topic)
	if err != nil {
		t.Fatalf("XReadGroupPending failed: %v", err)
	}
	want := []*MsgEntity{{MsgID: msgID, Key: "test_key", Val: "test_val"}}
	if !reflect.DeepEqual(msgs, want) {
		t.Errorf("XReadGroupPending() = %+v, want %+v", msgs, want)
	}

	// another consumer of the group does not see the pending message
	msgs, err = client.XReadGroupPending(ctx, group, "test_consumer2", topic)
	if !errors.Is(err, ErrNoMsg) {
		t.Errorf("XReadGroupPending() = %+v, err = %v, want ErrNoMsg", msgs, err)
	}
}

func Test_set_get_del_incr(t *testing.T) {
	skipWithoutRedis(t)

	client := NewClient(network, testAddress, testPassword)
	defer client.cli.Close()

	ctx := context.Background()
	keyPrefix := fmt.Sprintf("redmq_test_key_%d", time.Now().UnixNano())
	key := keyPrefix + "_main"

	if n, err := client.Set(ctx, key, "test_value"); err != nil || n != 1 {
		t.Errorf("Set() = %d, %v, want 1, nil", n, err)
	}

	val, err := client.Get(ctx, key)
	if err != nil || val != "test_value" {
		t.Errorf("Get() = %q, %v, want %q, nil", val, err, "test_value")
	}

	if n, err := client.Incr(ctx, keyPrefix+"_counter"); err != nil || n != 1 {
		t.Errorf("Incr() = %d, %v, want 1, nil", n, err)
	}

	if n, err := client.SetNX(ctx, key, "other"); err != nil || n != 0 {
		t.Errorf("SetNX() on existing key = %d, %v, want 0, nil", n, err)
	}

	if n, err := client.SetNEX(ctx, keyPrefix+"_nex", "value", 10); err != nil || n != 1 {
		t.Errorf("SetNEX() = %d, %v, want 1, nil", n, err)
	}

	// SetNEX on an existing key does not overwrite it
	if n, err := client.SetNEX(ctx, key, "other", 10); err != nil || n != 0 {
		t.Errorf("SetNEX() on existing key = %d, %v, want 0, nil", n, err)
	}

	val, err = client.Get(ctx, key)
	if err != nil || val != "test_value" {
		t.Errorf("Get() = %q, %v, want %q, nil", val, err, "test_value")
	}

	if err := client.Del(ctx, key); err != nil {
		t.Errorf("Del() error = %v", err)
	}
}

func Test_eval(t *testing.T) {
	skipWithoutRedis(t)

	client := NewClient(network, testAddress, testPassword)
	defer client.cli.Close()

	ctx := context.Background()
	key := fmt.Sprintf("redmq_test_eval_key_%d", time.Now().UnixNano())

	res, err := client.Eval(ctx, "return redis.call('GET', KEYS[1])", 1, []any{key})
	if !errors.Is(err, goredis.Nil) {
		t.Errorf("Eval() on missing key = %v, %v, want redis.Nil", res, err)
	}

	if _, err := client.Set(ctx, key, "lua"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	res, err = client.Eval(ctx, "return redis.call('GET', KEYS[1])", 1, []any{key})
	if err != nil || res != "lua" {
		t.Errorf("Eval() = %v, %v, want lua, nil", res, err)
	}

	if err := client.Del(ctx, key); err != nil {
		t.Errorf("Del() error = %v", err)
	}
}
