package redmq

import "time"

// ProducerOptions holds the optional configuration of a Producer.
type ProducerOptions struct {
	// maximum number of entries kept in the topic stream
	msgQueueLen int
}

// ProducerOption configures optional Producer settings.
type ProducerOption func(opts *ProducerOptions)

// WithMsgQueueLen sets the maximum number of entries kept in the topic stream.
func WithMsgQueueLen(len int) ProducerOption {
	return func(opts *ProducerOptions) {
		opts.msgQueueLen = len
	}
}

// repairProducer replaces invalid option values with defaults.
func repairProducer(opts *ProducerOptions) {
	if opts.msgQueueLen <= 0 {
		opts.msgQueueLen = 500
	}
}

// ConsumerOptions holds the optional configuration of a Consumer.
type ConsumerOptions struct {
	// timeout of each message receive round
	receiveTimeout time.Duration
	// maximum number of times a message may fail before it is delivered to
	// the dead letter mailbox
	maxRetryLimit int
	// dead letter mailbox, can be implemented by the user
	deadLetterMailbox DeadLetterMailbox
	// timeout of the dead letter delivery flow
	deadLetterDeliverTimeout time.Duration
	// timeout of the message handling flow
	handleMsgsTimeout time.Duration
}

// ConsumerOption configures optional Consumer settings.
type ConsumerOption func(opts *ConsumerOptions)

// WithReceiveTimeout sets the timeout of each message receive round.
func WithReceiveTimeout(timeout time.Duration) ConsumerOption {
	return func(opts *ConsumerOptions) {
		opts.receiveTimeout = timeout
	}
}

// WithMaxRetryLimit sets the maximum number of times a message may fail
// before it is delivered to the dead letter mailbox.
func WithMaxRetryLimit(maxRetryLimit int) ConsumerOption {
	return func(opts *ConsumerOptions) {
		opts.maxRetryLimit = maxRetryLimit
	}
}

// WithDeadLetterMailbox injects a custom dead letter mailbox implementation.
func WithDeadLetterMailbox(mailbox DeadLetterMailbox) ConsumerOption {
	return func(opts *ConsumerOptions) {
		opts.deadLetterMailbox = mailbox
	}
}

// WithDeadLetterDeliverTimeout sets the timeout of the dead letter delivery flow.
func WithDeadLetterDeliverTimeout(timeout time.Duration) ConsumerOption {
	return func(opts *ConsumerOptions) {
		opts.deadLetterDeliverTimeout = timeout
	}
}

// WithHandleMsgsTimeout sets the timeout of the message handling flow.
func WithHandleMsgsTimeout(timeout time.Duration) ConsumerOption {
	return func(opts *ConsumerOptions) {
		opts.handleMsgsTimeout = timeout
	}
}

// repairConsumer replaces invalid option values with defaults.
func repairConsumer(opts *ConsumerOptions) {
	if opts.receiveTimeout <= 0 {
		opts.receiveTimeout = 2 * time.Second
	}

	if opts.maxRetryLimit <= 0 {
		opts.maxRetryLimit = 3
	}

	if opts.deadLetterMailbox == nil {
		opts.deadLetterMailbox = NewDeadLetterLogger()
	}

	if opts.deadLetterDeliverTimeout <= 0 {
		opts.deadLetterDeliverTimeout = time.Second
	}

	if opts.handleMsgsTimeout <= 0 {
		opts.handleMsgsTimeout = time.Second
	}
}
