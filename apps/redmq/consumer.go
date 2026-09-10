package redmq

import (
	"context"
	"errors"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/log"
	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

// Errors returned by NewConsumer when the given parameters are invalid.
var (
	errNilCallback    = errors.New("callback function can't be empty")
	errNilRedisClient = errors.New("redis client can't be empty")
	errEmptyStreamIDs = errors.New("topic | group_id | consumer_id can't be empty")
)

// MsgCallback is invoked for every message received by a Consumer.
// It is defined by the user of this library.
type MsgCallback func(ctx context.Context, msg *redis.MsgEntity) error

// Consumer consumes messages from a Redis stream topic as a member of a
// consumer group.
type Consumer struct {
	// consumer lifecycle management
	ctx  context.Context
	stop context.CancelFunc

	// callback invoked for every received message, defined by the user
	callbackFunc MsgCallback

	// redis client, used to build the message queue on top of Redis
	client *redis.Client

	// consumed topic
	topic string
	// the consumer group this consumer belongs to
	groupID string
	// the id of this consumer within the group
	consumerID string

	// accumulated failure count per message
	failureCnts map[redis.MsgEntity]int

	// user defined configuration
	opts *ConsumerOptions
}

// NewConsumer validates the parameters, applies the options and starts the
// consume loop in the background.
func NewConsumer(client *redis.Client, topic, groupID, consumerID string, callbackFunc MsgCallback, opts ...ConsumerOption) (*Consumer, error) {

	ctx, stop := context.WithCancel(context.Background())
	c := Consumer{
		client:       client,
		ctx:          ctx,
		stop:         stop,
		callbackFunc: callbackFunc,
		topic:        topic,
		groupID:      groupID,
		consumerID:   consumerID,

		opts: &ConsumerOptions{},

		failureCnts: make(map[redis.MsgEntity]int),
	}

	if err := c.checkParam(); err != nil {
		stop()
		return nil, err
	}

	for _, opt := range opts {
		opt(c.opts)
	}

	repairConsumer(c.opts)

	go c.run()
	return &c, nil
}

func (c *Consumer) checkParam() error {
	if c.callbackFunc == nil {
		return errNilCallback
	}

	if c.client == nil {
		return errNilRedisClient
	}

	if c.topic == "" || c.consumerID == "" || c.groupID == "" {
		return errEmptyStreamIDs
	}

	return nil
}

// Stop terminates the consume loop.
func (c *Consumer) Stop() {
	c.stop()
}

// run is the main consume loop.
func (c *Consumer) run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// receive and handle newly published messages
		msgs, err := c.receive()
		if err != nil {
			log.ErrorContextf(c.ctx, "receive msg failed, err: %v", err)
			continue
		}

		tctx, cancel := context.WithTimeout(c.ctx, c.opts.handleMsgsTimeout)
		c.handleMsgs(tctx, msgs)
		cancel()

		// deliver dead letters
		tctx, cancel = context.WithTimeout(c.ctx, c.opts.deadLetterDeliverTimeout)
		c.deliverDeadLetter(tctx)
		cancel()

		// receive and handle pending (delivered but unacknowledged) messages
		pendingMsgs, err := c.receivePending()
		if err != nil {
			log.ErrorContextf(c.ctx, "pending msg received failed, err: %v", err)
			continue
		}

		tctx, cancel = context.WithTimeout(c.ctx, c.opts.handleMsgsTimeout)
		c.handleMsgs(tctx, pendingMsgs)
		cancel()
	}
}

func (c *Consumer) receive() ([]*redis.MsgEntity, error) {
	msgs, err := c.client.XReadGroup(c.ctx, c.groupID, c.consumerID, c.topic, int(c.opts.receiveTimeout.Milliseconds()))
	if err != nil && !errors.Is(err, redis.ErrNoMsg) {
		return nil, err
	}

	return msgs, nil
}

func (c *Consumer) receivePending() ([]*redis.MsgEntity, error) {
	pendingMsgs, err := c.client.XReadGroupPending(c.ctx, c.groupID, c.consumerID, c.topic)
	if err != nil && !errors.Is(err, redis.ErrNoMsg) {
		return nil, err
	}

	return pendingMsgs, nil
}

func (c *Consumer) handleMsgs(ctx context.Context, msgs []*redis.MsgEntity) {
	for _, msg := range msgs {
		if err := c.callbackFunc(ctx, msg); err != nil {
			// increment the failure counter
			c.failureCnts[*msg]++
			continue
		}

		// the callback succeeded, acknowledge the message
		if err := c.client.XACK(ctx, c.topic, c.groupID, msg.MsgID); err != nil {
			log.ErrorContextf(ctx, "msg ack failed, msg id: %s, err: %v", msg.MsgID, err)
			continue
		}

		delete(c.failureCnts, *msg)
	}
}

func (c *Consumer) deliverDeadLetter(ctx context.Context) {
	// messages that failed the configured number of times are delivered to
	// the dead letter mailbox and then acknowledged
	for msg, failureCnt := range c.failureCnts {
		if failureCnt < c.opts.maxRetryLimit {
			continue
		}

		// deliver to the dead letter queue
		if err := c.opts.deadLetterMailbox.Deliver(ctx, &msg); err != nil {
			log.ErrorContextf(c.ctx, "dead letter deliver failed, msg id: %s, err: %v", msg.MsgID, err)
		}

		// acknowledge the message
		if err := c.client.XACK(ctx, c.topic, c.groupID, msg.MsgID); err != nil {
			log.ErrorContextf(c.ctx, "msg ack failed, msg id: %s, err: %v", msg.MsgID, err)
			continue
		}

		// messages acknowledged successfully are removed from the failure map
		delete(c.failureCnts, msg)
	}
}
