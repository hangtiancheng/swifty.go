// Package redis provides the Redis client wrapper used by redmq to talk to
// Redis streams. It is built on top of github.com/redis/go-redis/v9.
package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// MsgEntity is a single message entry read from a Redis stream.
type MsgEntity struct {
	MsgID string
	Key   string
	Val   string
}

// ErrNoMsg is returned when no message was received from the stream.
var ErrNoMsg = errors.New("no msg received")

// ErrInvalidXReadGroupArgs is returned when the XREADGROUP arguments are empty.
var ErrInvalidXReadGroupArgs = errors.New("redis XREADGROUP groupID/consumerID/topic can't be empty")

// Client is a Redis client used to produce and consume messages.
type Client struct {
	opts *ClientOptions
	cli  *goredis.Client
}

// NewClient creates a Redis client with the given network, address and
// password. Optional pool behaviour can be configured with ClientOption.
func NewClient(network, address, password string, opts ...ClientOption) *Client {
	c := Client{
		opts: &ClientOptions{
			network:  network,
			address:  address,
			password: password,
		},
	}

	for _, opt := range opts {
		opt(c.opts)
	}

	repairClient(c.opts)

	return &Client{
		opts: c.opts,
		cli:  goredis.NewClient(c.opts.toGoRedisOptions()),
	}
}

// Close closes the underlying Redis connection pool.
func (c *Client) Close() error {
	return c.cli.Close()
}

// XADD appends one message to the topic stream and trims the stream to at
// most maxLen entries. It returns the message id assigned by Redis.
func (c *Client) XADD(ctx context.Context, topic string, maxLen int, key, val string) (string, error) {
	if topic == "" {
		return "", errors.New("redis XADD topic can't be empty")
	}

	return c.cli.XAdd(ctx, &goredis.XAddArgs{
		Stream: topic,
		MaxLen: int64(maxLen),
		Values: []interface{}{key, val},
	}).Result()
}

// XACK acknowledges the message with the given id on behalf of the
// consumer group. It returns an error when Redis did not acknowledge
// exactly one message.
func (c *Client) XACK(ctx context.Context, topic, groupID, msgID string) error {
	if topic == "" || groupID == "" || msgID == "" {
		return errors.New("redis XACK topic | group id | msg id can't be empty")
	}

	reply, err := c.cli.XAck(ctx, topic, groupID, msgID).Result()
	if err != nil {
		return err
	}
	if reply != 1 {
		return fmt.Errorf("invalid reply: %d", reply)
	}

	return nil
}

// XReadGroupPending reads messages that were already delivered to this
// consumer but not yet acknowledged.
func (c *Client) XReadGroupPending(ctx context.Context, groupID, consumerID, topic string) ([]*MsgEntity, error) {
	return c.xReadGroup(ctx, groupID, consumerID, topic, 0, true)
}

// XReadGroup reads brand new messages never delivered to the group, blocking
// at most timeoutMilliSeconds. A non positive timeout blocks forever, which
// matches the Redis BLOCK 0 semantics.
func (c *Client) XReadGroup(ctx context.Context, groupID, consumerID, topic string, timeoutMilliSeconds int) ([]*MsgEntity, error) {
	return c.xReadGroup(ctx, groupID, consumerID, topic, timeoutMilliSeconds, false)
}

func (c *Client) xReadGroup(ctx context.Context, groupID, consumerID, topic string, timeoutMilliSeconds int, pending bool) ([]*MsgEntity, error) {
	if groupID == "" || consumerID == "" || topic == "" {
		return nil, ErrInvalidXReadGroupArgs
	}

	args := &goredis.XReadGroupArgs{
		Group:    groupID,
		Consumer: consumerID,
		// ">" fetches new messages that were never delivered to the group.
		Streams: []string{topic, ">"},
		Block:   time.Duration(timeoutMilliSeconds) * time.Millisecond,
	}
	if pending {
		// "0" fetches messages assigned to this consumer but not yet
		// acknowledged, starting from the smallest message id (0-0).
		args.Streams = []string{topic, "0"}
		args.Block = -1 // do not send the BLOCK option
	}

	streams, err := c.cli.XReadGroup(ctx, args).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, ErrNoMsg
		}
		return nil, err
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, ErrNoMsg
	}

	return toMsgEntities(streams[0].Messages), nil
}

// Get returns the string value stored at key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("redis GET key can't be empty")
	}

	return c.cli.Get(ctx, key).Result()
}

// Set stores the value at key without expiration. It returns 1 on success.
func (c *Client) Set(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key or value can't be empty")
	}

	status, err := c.cli.Set(ctx, key, value, 0).Result()
	if err != nil {
		return -1, err
	}
	if strings.EqualFold(status, "OK") {
		return 1, nil
	}

	return -1, fmt.Errorf("invalid reply: %s", status)
}

// SetNEX stores the value at key with an expiration in seconds, only if the
// key does not exist yet. It returns 1 when the key was set, 0 otherwise.
func (c *Client) SetNEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key NX EX or value can't be empty")
	}

	ok, err := c.cli.SetNX(ctx, key, value, time.Duration(expireSeconds)*time.Second).Result()
	if err != nil {
		return -1, err
	}
	if ok {
		return 1, nil
	}

	return 0, nil
}

// SetNX stores the value at key, only if the key does not exist yet. It
// returns 1 when the key was set, 0 otherwise.
func (c *Client) SetNX(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key NX or value can't be empty")
	}

	ok, err := c.cli.SetNX(ctx, key, value, 0).Result()
	if err != nil {
		return -1, err
	}
	if ok {
		return 1, nil
	}

	return 0, nil
}

// Del removes the key from Redis.
func (c *Client) Del(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("redis DEL key can't be empty")
	}

	_, err := c.cli.Del(ctx, key).Result()
	return err
}

// Incr increments the integer stored at key by one and returns the new value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return -1, errors.New("redis INCR key can't be empty")
	}

	return c.cli.Incr(ctx, key).Result()
}

// Eval runs the given Lua script on the server. keysAndArgs holds keyCount
// key names followed by the additional script arguments.
func (c *Client) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	if keyCount < 0 {
		return nil, errors.New("redis EVAL key count can't be negative")
	}
	if keyCount > len(keysAndArgs) {
		return nil, fmt.Errorf("redis EVAL expected %d keys, got %d", keyCount, len(keysAndArgs))
	}

	keys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		keys[i] = toString(keysAndArgs[i])
	}
	args := make([]interface{}, len(keysAndArgs)-keyCount)
	copy(args, keysAndArgs[keyCount:])

	return goredis.NewScript(src).Run(ctx, c.cli, keys, args...).Result()
}

// XGroupCreate creates a consumer group for the topic stream, starting from
// the smallest message id.
func (c *Client) XGroupCreate(ctx context.Context, topic, group string) (string, error) {
	return c.cli.XGroupCreate(ctx, topic, group, "0-0").Result()
}

// toMsgEntities converts raw stream messages into message entities. Each
// entity carries the first field/value pair of the stream entry.
func toMsgEntities(msgs []goredis.XMessage) []*MsgEntity {
	entities := make([]*MsgEntity, 0, len(msgs))
	for _, msg := range msgs {
		entity := &MsgEntity{MsgID: msg.ID}
		for k, v := range msg.Values {
			entity.Key = k
			entity.Val = toString(v)
			break
		}
		entities = append(entities, entity)
	}

	return entities
}

// toString converts a Redis reply value to its string representation.
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprint(val)
	}
}
