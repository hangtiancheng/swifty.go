package redmq

import (
	"context"
	"testing"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

func TestNewDeadLetterLogger(t *testing.T) {
	mailbox := NewDeadLetterLogger()
	if mailbox == nil {
		t.Fatal("NewDeadLetterLogger() = nil, want a non nil mailbox")
	}

	msg := &redis.MsgEntity{MsgID: "1692066364494-0", Key: "test_key", Val: "test_val"}
	if err := mailbox.Deliver(context.Background(), msg); err != nil {
		t.Errorf("Deliver() error = %v, want nil", err)
	}
}

func TestNewProducer(t *testing.T) {
	producer := NewProducer(nil, WithMsgQueueLen(10))
	if producer == nil {
		t.Fatal("NewProducer() = nil, want a non nil producer")
	}
	if got := producer.opts.msgQueueLen; got != 10 {
		t.Errorf("NewProducer() msgQueueLen = %d, want 10", got)
	}

	// an invalid queue length falls back to the default
	producer = NewProducer(nil)
	if got := producer.opts.msgQueueLen; got != 500 {
		t.Errorf("NewProducer() default msgQueueLen = %d, want 500", got)
	}
}
