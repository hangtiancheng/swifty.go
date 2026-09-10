package example_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

const (
	network  = "tcp"
	address  = "127.0.0.1:6379"
	password = ""
)

// bootstrapKey is the message key used to create the topic stream before the
// consumer group exists.
const bootstrapKey = "bootstrap"

// waitForMsg waits for a message with the wanted key and value on the given
// channel, ignoring bootstrap messages, and fails the test on timeout.
func waitForMsg(t *testing.T, ch <-chan *redis.MsgEntity, wantKey, wantVal string) {
	t.Helper()

	timeout := time.After(5 * time.Second)
	for {
		select {
		case msg := <-ch:
			if msg.Key == bootstrapKey {
				continue
			}
			if msg.Key != wantKey || msg.Val != wantVal {
				t.Errorf("received msg = %+v, want key %s and val %s", msg, wantKey, wantVal)
			}
			return
		case <-timeout:
			t.Fatalf("timed out waiting for msg with key %s and val %s", wantKey, wantVal)
		}
	}
}

// skipWithoutRedis skips the test when no Redis server is reachable at
// localhost:6379.
func skipWithoutRedis(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout(network, address, 500*time.Millisecond)
	if err != nil {
		t.Skipf("skipping: no redis server reachable at %s: %v", address, err)
	}
	_ = conn.Close()
}

// newTopicName returns a unique topic name so test runs do not interfere
// with each other.
func newTopicName(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// createGroup creates the consumer group for the topic, ignoring the
// BUSYGROUP error when the group already exists. The topic stream must exist
// before the group can be created, so a bootstrap message is written first.
func createGroup(t *testing.T, client *redis.Client, topic, group string) {
	t.Helper()

	ctx := context.Background()
	if _, err := client.XADD(ctx, topic, 100, "bootstrap", "bootstrap"); err != nil {
		t.Fatalf("XADD failed: %v", err)
	}

	if _, err := client.XGroupCreate(ctx, topic, group); err != nil {
		const busyGroup = "BUSYGROUP"
		if len(err.Error()) >= len(busyGroup) && err.Error()[:len(busyGroup)] == busyGroup {
			return
		}
		t.Fatalf("XGroupCreate failed: %v", err)
	}
}
