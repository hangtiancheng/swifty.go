# redmq

<p align="center">
  <b>redmq: a message queue built purely on Redis</b>
</p>

## Introduction

Before using this SDK, it is recommended to learn the features of Redis streams first.
<a href="https://redis.io/docs/data-types/streams/">Redis streams</a>

## How `redmq` works

A message queue built on Redis

## Getting started

Users need to create the topic and the consumer group first.

- Create the topic: `my_test_topic`

```redis
127.0.0.1:6379> xadd my_test_topic * first_key first_val
"1692066364494-0"
```

- Create the consumer group

```redis
127.0.0.1:6379> XGROUP CREATE my_test_topic my_test_group 0-0
OK
```

- Build the Redis client

```go
import "github.com/hangtiancheng/swifty.go/apps/redmq"

func main() {
    redisClient := redmq.NewRedisClient("tcp", "my_address", "my_password")
    // ...
}
```

- Start a producer

```go
import (
	"context"

	"github.com/hangtiancheng/swifty.go/apps/redmq"
)

func main() {
    // ...
    producer := redmq.NewProducer(redisClient, redmq.WithMsgQueueLen(10))
    ctx := context.Background()
    msgID, err := producer.SendMsg(ctx, topic, "test_kk", "test_vv")
}
```

- Start a consumer

```go
import (
	"github.com/hangtiancheng/swifty.go/apps/redmq"
)

func main() {
  // ...
  // build and start the consumer
  consumer, err := redmq.NewConsumer(redisClient, topic, consumerGroup, consumerID, callbackFunc,
    // each message may fail at most twice before it is dead lettered
    redmq.WithMaxRetryLimit(2),
    // each receive round times out after 2 seconds
    redmq.WithReceiveTimeout(2*time.Second),
    // inject a custom dead letter mailbox
    redmq.WithDeadLetterMailbox(demoDeadLetterMailbox))
  if err != nil {
    panic(err)
  }
  defer consumer.Stop()
}
```

## Usage examples

Complete usage examples can be found in the `example` package:

- Mock the producer publishing flow

```go
import (
	"context"
	"testing"

	"github.com/hangtiancheng/swifty.go/apps/redmq"
)

func TestProducer(t *testing.T) {
	client := redmq.NewRedisClient("tcp", "127.0.0.1:6379", "")
	// keep at most ten messages in the stream
	producer := redmq.NewProducer(client, redmq.WithMsgQueueLen(10))
	ctx := context.Background()
	msgID, err := producer.SendMsg(ctx, topic, "test_kk", "test_vv")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(msgID)
}
```

- Mock the consumer consuming flow

```go
import (
	"context"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/redmq"
	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

// a custom dead letter mailbox implementation
type DemoDeadLetterMailbox struct {
	do func(msg *redis.MsgEntity)
}

func NewDemoDeadLetterMailbox(do func(msg *redis.MsgEntity)) *DemoDeadLetterMailbox {
	return &DemoDeadLetterMailbox{
		do: do,
	}
}

// the handling method for messages received by the dead letter mailbox
func (d *DemoDeadLetterMailbox) Deliver(ctx context.Context, msg *redis.MsgEntity) error {
	d.do(msg)
	return nil
}

func TestConsumer(t *testing.T) {
	client := redmq.NewRedisClient("tcp", "127.0.0.1:6379", "")

	// the callback invoked for every received message
	callbackFunc := func(ctx context.Context, msg *redis.MsgEntity) error {
		t.Logf("receive msg, msg id: %s, msg key: %s, msg val: %s", msg.MsgID, msg.Key, msg.Val)
		return nil
	}

	// build and start the consumer
	consumer, err := redmq.NewConsumer(client, topic, consumerGroup, consumerID, callbackFunc,
		// each message may fail at most twice before it is dead lettered
		redmq.WithMaxRetryLimit(2),
		// each receive round times out after 2 seconds
		redmq.WithReceiveTimeout(2*time.Second),
		// inject a custom dead letter mailbox
		redmq.WithDeadLetterMailbox(NewDemoDeadLetterMailbox(func(msg *redis.MsgEntity) {
			t.Logf("receive dead letter, msg id: %s, msg key: %s, msg val: %s", msg.MsgID, msg.Key, msg.Val)
		})))
	if err != nil {
		t.Error(err)
		return
	}
	defer consumer.Stop()

	// run the consume loop for a while
	<-time.After(10 * time.Second)
}
```

## Dependencies

- [github.com/redis/go-redis/v9](https://github.com/redis/go-redis) — Redis client (replaces `gomodule/redigo`)
- [go.uber.org/zap](https://go.uber.org/zap) — structured logging to standard output

## Project layout

```
.
├── producer.go        # Producer and SendMsg
├── consumer.go        # Consumer, MsgCallback and the consume loop
├── dead_letter.go     # DeadLetterMailbox interface and the default logger mailbox
├── option.go          # producer/consumer options
├── client.go          # public Redis client re-exports
├── internal/
│   ├── log/           # zap based stdout logging used by redmq
│   └── redis/         # go-redis v9 based Redis client wrapper
└── example/           # runnable usage examples (as tests)
```

## Testing

The example and Redis integration tests require a Redis server reachable at
`localhost:6379`; they are skipped automatically when no server is present.

```bash
go test ./...
```
