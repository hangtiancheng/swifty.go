package redmq

import (
	"context"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

// Producer publishes messages to Redis stream based topics.
type Producer struct {
	client *redis.Client
	opts   *ProducerOptions
}

// NewProducer creates a Producer that publishes messages through client.
func NewProducer(client *redis.Client, opts ...ProducerOption) *Producer {
	p := Producer{
		client: client,
		opts:   &ProducerOptions{},
	}

	for _, opt := range opts {
		opt(p.opts)
	}

	repairProducer(p.opts)

	return &p
}

// SendMsg publishes one message to the topic and returns the message id
// assigned by Redis.
func (p *Producer) SendMsg(ctx context.Context, topic, key, val string) (string, error) {
	return p.client.XADD(ctx, topic, p.opts.msgQueueLen, key, val)
}
