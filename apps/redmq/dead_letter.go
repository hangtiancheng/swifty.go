package redmq

import (
	"context"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

// DeadLetterMailbox receives messages whose processing failed the configured
// number of times.
type DeadLetterMailbox interface {
	Deliver(ctx context.Context, msg *redis.MsgEntity) error
}

// DeadLetterLogger is the default dead letter mailbox; it only logs
// information about failed messages.
type DeadLetterLogger struct{}

// NewDeadLetterLogger creates the default dead letter mailbox.
func NewDeadLetterLogger() *DeadLetterLogger {
	return &DeadLetterLogger{}
}

// Deliver logs the failed message.
func (d *DeadLetterLogger) Deliver(ctx context.Context, msg *redis.MsgEntity) error {
	log.ErrorContextf(ctx, "msg failed exceeded retry limit, msg id: %s", msg.MsgID)
	return nil
}
