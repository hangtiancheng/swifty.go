package example_test

import (
	"context"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/redmq"
	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

// DemoDeadLetterMailbox is a custom dead letter mailbox implementation that
// forwards failed messages to the given function.
type DemoDeadLetterMailbox struct {
	do func(msg *redis.MsgEntity)
}

// NewDemoDeadLetterMailbox creates a dead letter mailbox that calls do for
// every delivered message.
func NewDemoDeadLetterMailbox(do func(msg *redis.MsgEntity)) *DemoDeadLetterMailbox {
	return &DemoDeadLetterMailbox{
		do: do,
	}
}

// Deliver handles the message received by the dead letter mailbox.
func (d *DemoDeadLetterMailbox) Deliver(ctx context.Context, msg *redis.MsgEntity) error {
	d.do(msg)
	return nil
}

func TestConsumer(t *testing.T) {
	skipWithoutRedis(t)

	client := redmq.NewRedisClient(network, address, password)
	defer client.Close()

	topic := newTopicName("redmq_example_topic")
	group := "my_test_group"
	createGroup(t, client, topic, group)

	msgCh := make(chan *redis.MsgEntity, 16)

	// the callback runs for every received message
	callbackFunc := func(ctx context.Context, msg *redis.MsgEntity) error {
		t.Logf("receive msg, msg id: %s, msg key: %s, msg val: %s", msg.MsgID, msg.Key, msg.Val)
		msgCh <- msg
		return nil
	}

	// build and start the consumer
	consumer, err := redmq.NewConsumer(client, topic, group, "my_consumer", callbackFunc,
		// each message may fail at most twice before it is dead lettered
		redmq.WithMaxRetryLimit(2),
		// each receive round times out after 2 seconds
		redmq.WithReceiveTimeout(2*time.Second),
		// inject the custom dead letter mailbox
		redmq.WithDeadLetterMailbox(NewDemoDeadLetterMailbox(func(msg *redis.MsgEntity) {
			t.Logf("receive dead letter, msg id: %s, msg key: %s, msg val: %s", msg.MsgID, msg.Key, msg.Val)
		})))
	if err != nil {
		t.Errorf("NewConsumer() error = %v", err)
		return
	}
	defer consumer.Stop()

	producer := redmq.NewProducer(client)
	if _, err := producer.SendMsg(context.Background(), topic, "test_kk", "test_vv"); err != nil {
		t.Errorf("SendMsg() error = %v", err)
		return
	}

	waitForMsg(t, msgCh, "test_kk", "test_vv")
}

func TestConsumerDeadLetter(t *testing.T) {
	skipWithoutRedis(t)

	client := redmq.NewRedisClient(network, address, password)
	defer client.Close()

	topic := newTopicName("redmq_example_topic")
	group := "my_test_group"
	createGroup(t, client, topic, group)

	deadLetterCh := make(chan *redis.MsgEntity, 16)

	// the callback always fails, so the message ends up in the dead letter
	// mailbox after the first failure
	callbackFunc := func(ctx context.Context, msg *redis.MsgEntity) error {
		return context.DeadlineExceeded
	}

	// build and start the consumer with a retry limit of one
	consumer, err := redmq.NewConsumer(client, topic, group, "my_consumer", callbackFunc,
		redmq.WithMaxRetryLimit(1),
		redmq.WithReceiveTimeout(2*time.Second),
		redmq.WithDeadLetterMailbox(NewDemoDeadLetterMailbox(func(msg *redis.MsgEntity) {
			deadLetterCh <- msg
		})))
	if err != nil {
		t.Errorf("NewConsumer() error = %v", err)
		return
	}
	defer consumer.Stop()

	producer := redmq.NewProducer(client)
	if _, err := producer.SendMsg(context.Background(), topic, "dead_kk", "dead_vv"); err != nil {
		t.Errorf("SendMsg() error = %v", err)
		return
	}

	waitForMsg(t, deadLetterCh, "dead_kk", "dead_vv")
}
