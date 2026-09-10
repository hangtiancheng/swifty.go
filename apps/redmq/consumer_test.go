package redmq

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

const (
	testNetwork  = "tcp"
	testAddress  = "127.0.0.1:6379"
	testPassword = ""
)

// skipWithoutRedis skips the test when no Redis server is reachable at
// localhost:6379.
func skipWithoutRedis(t *testing.T) {
	t.Helper()

	conn, err := net.DialTimeout(testNetwork, testAddress, 500*time.Millisecond)
	if err != nil {
		t.Skipf("skipping: no redis server reachable at %s: %v", testAddress, err)
	}
	_ = conn.Close()
}

// stubMailbox is a dead letter mailbox used by the tests.
type stubMailbox struct {
	deliverErr error
	delivered  []*redis.MsgEntity
}

func (m *stubMailbox) Deliver(ctx context.Context, msg *redis.MsgEntity) error {
	if m.deliverErr != nil {
		return m.deliverErr
	}
	m.delivered = append(m.delivered, msg)
	return nil
}

// newTestConsumer builds a Consumer without starting its run loop.
func newTestConsumer(t *testing.T, callbackFunc MsgCallback, mailbox DeadLetterMailbox) *Consumer {
	t.Helper()

	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	return &Consumer{
		ctx:          ctx,
		stop:         stop,
		callbackFunc: callbackFunc,
		topic:        "test_topic",
		groupID:      "test_group",
		consumerID:   "test_consumer",
		failedMsgs:   make(map[string]*failedMsg),
		opts: &ConsumerOptions{
			maxRetryLimit:     2,
			deadLetterMailbox: mailbox,
		},
	}
}

func Test_consumer_handleMsgs_counts_failures_per_message_id(t *testing.T) {
	// the callback always fails, so the consumer never talks to Redis and a
	// nil client is safe here
	consumer := newTestConsumer(t, func(ctx context.Context, msg *redis.MsgEntity) error {
		return errors.New("processing failed")
	}, &stubMailbox{})

	// two distinct stream entries carrying identical content
	msg1 := &redis.MsgEntity{MsgID: "1-1", Key: "k", Val: "v"}
	msg2 := &redis.MsgEntity{MsgID: "1-2", Key: "k", Val: "v"}

	consumer.handleMsgs(context.Background(), []*redis.MsgEntity{msg1, msg2})
	if got := consumer.failedMsgs["1-1"].failures; got != 1 {
		t.Errorf("failure count of msg 1-1 = %d, want 1", got)
	}
	if got := consumer.failedMsgs["1-2"].failures; got != 1 {
		t.Errorf("failure count of msg 1-2 = %d, want 1", got)
	}

	consumer.handleMsgs(context.Background(), []*redis.MsgEntity{msg1})
	if got := consumer.failedMsgs["1-1"].failures; got != 2 {
		t.Errorf("failure count of msg 1-1 = %d, want 2", got)
	}
	if got := consumer.failedMsgs["1-2"].failures; got != 1 {
		t.Errorf("failure count of msg 1-2 = %d, want 1", got)
	}
}

func Test_consumer_deliverDeadLetter_failed_delivery_is_not_acked(t *testing.T) {
	// the mailbox keeps failing, so the message must neither be acknowledged
	// nor dropped; a nil client is safe because XACK must not be reached
	consumer := newTestConsumer(t, func(ctx context.Context, msg *redis.MsgEntity) error {
		return errors.New("processing failed")
	}, &stubMailbox{deliverErr: errors.New("mailbox unavailable")})

	msg := &redis.MsgEntity{MsgID: "1-1", Key: "k", Val: "v"}
	consumer.handleMsgs(context.Background(), []*redis.MsgEntity{msg, msg})

	consumer.deliverDeadLetter(context.Background())

	if len(consumer.failedMsgs) != 1 {
		t.Errorf("failedMsgs = %+v, want the message to be kept for retry", consumer.failedMsgs)
	}
}

func Test_consumer_deliverDeadLetter_below_retry_limit(t *testing.T) {
	mailbox := &stubMailbox{deliverErr: errors.New("mailbox unavailable")}
	consumer := newTestConsumer(t, func(ctx context.Context, msg *redis.MsgEntity) error {
		return errors.New("processing failed")
	}, mailbox)

	msg := &redis.MsgEntity{MsgID: "1-1", Key: "k", Val: "v"}
	consumer.handleMsgs(context.Background(), []*redis.MsgEntity{msg})

	consumer.deliverDeadLetter(context.Background())

	if len(mailbox.delivered) != 0 {
		t.Errorf("Deliver() called %d times, want 0 before the retry limit is reached", len(mailbox.delivered))
	}
}

func Test_consumer_success_and_dead_letter_flow(t *testing.T) {
	skipWithoutRedis(t)

	client := NewRedisClient(testNetwork, testAddress, testPassword)
	defer client.Close()

	ctx := context.Background()
	topic := fmt.Sprintf("redmq_test_topic_%d", time.Now().UnixNano())
	group := fmt.Sprintf("redmq_test_group_%d", time.Now().UnixNano())

	// the first message creates the topic stream, and the consumer group is
	// created afterwards starting from 0-0
	if _, err := client.XADD(ctx, topic, 100, "k", "v1"); err != nil {
		t.Fatalf("XADD failed: %v", err)
	}
	if _, err := client.XGroupCreate(ctx, topic, group); err != nil {
		t.Fatalf("XGroupCreate failed: %v", err)
	}

	// failCallback switches the callback between the success and failure flows
	failCallback := false
	mailbox := &stubMailbox{}
	consumer := newTestConsumer(t, func(ctx context.Context, msg *redis.MsgEntity) error {
		if failCallback {
			return errors.New("processing failed")
		}
		return nil
	}, mailbox)
	consumer.client = client
	consumer.topic = topic
	consumer.groupID = group
	consumer.opts.receiveTimeout = 2 * time.Second

	// a successful callback acknowledges the message
	msgs, err := consumer.receive()
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("receive() = %d msgs, want 1", len(msgs))
	}

	consumer.handleMsgs(ctx, msgs)

	if len(consumer.failedMsgs) != 0 {
		t.Errorf("failedMsgs = %+v, want an empty map after a successful callback", consumer.failedMsgs)
	}
	// receivePending swallows ErrNoMsg, so the pending entries list is
	// checked through the client directly
	pending, err := client.XReadGroupPending(ctx, group, "test_consumer", topic)
	if !errors.Is(err, redis.ErrNoMsg) {
		t.Errorf("XReadGroupPending() = %+v, err = %v, want ErrNoMsg after ack", pending, err)
	}

	// a failing callback dead letters the message once the retry limit is
	// reached, acknowledges it and clears the bookkeeping
	failCallback = true
	if _, err := client.XADD(ctx, topic, 100, "k", "v2"); err != nil {
		t.Fatalf("XADD failed: %v", err)
	}

	msgs, err = consumer.receive()
	if err != nil {
		t.Fatalf("receive failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("receive() = %d msgs, want 1", len(msgs))
	}
	msgID := msgs[0].MsgID

	for i := 0; i < consumer.opts.maxRetryLimit; i++ {
		consumer.handleMsgs(ctx, msgs)
	}

	consumer.deliverDeadLetter(ctx)

	if len(mailbox.delivered) != 1 || mailbox.delivered[0].MsgID != msgID {
		t.Errorf("dead letter mailbox = %+v, want the message %s", mailbox.delivered, msgID)
	}
	if _, ok := consumer.failedMsgs[msgID]; ok {
		t.Errorf("failedMsgs still contains %s, want it removed after the ack", msgID)
	}
	pending, err = client.XReadGroupPending(ctx, group, "test_consumer", topic)
	if !errors.Is(err, redis.ErrNoMsg) {
		t.Errorf("XReadGroupPending() = %+v, err = %v, want ErrNoMsg after dead letter ack", pending, err)
	}
}

func TestNewConsumer_param_validation(t *testing.T) {
	client := NewRedisClient("tcp", "127.0.0.1:6379", "")
	callback := func(ctx context.Context, msg *redis.MsgEntity) error { return nil }

	tests := []struct {
		name         string
		client       *redis.Client
		topic        string
		groupID      string
		consumerID   string
		callbackFunc MsgCallback
		want         error
	}{
		{
			name:         "nil callback",
			client:       client,
			topic:        "topic",
			groupID:      "group",
			consumerID:   "consumer",
			callbackFunc: nil,
			want:         errNilCallback,
		},
		{
			name:         "nil redis client",
			client:       nil,
			topic:        "topic",
			groupID:      "group",
			consumerID:   "consumer",
			callbackFunc: callback,
			want:         errNilRedisClient,
		},
		{
			name:         "empty topic",
			client:       client,
			topic:        "",
			groupID:      "group",
			consumerID:   "consumer",
			callbackFunc: callback,
			want:         errEmptyStreamIDs,
		},
		{
			name:         "empty group id",
			client:       client,
			topic:        "topic",
			groupID:      "",
			consumerID:   "consumer",
			callbackFunc: callback,
			want:         errEmptyStreamIDs,
		},
		{
			name:         "empty consumer id",
			client:       client,
			topic:        "topic",
			groupID:      "group",
			consumerID:   "",
			callbackFunc: callback,
			want:         errEmptyStreamIDs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.client, tt.topic, tt.groupID, tt.consumerID, tt.callbackFunc)
			if consumer != nil {
				defer consumer.Stop()
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("NewConsumer() error = %v, want %v", err, tt.want)
			}
		})
	}
}
