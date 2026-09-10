package example_test

import (
	"context"
	"testing"

	"github.com/hangtiancheng/swifty.go/apps/redmq"
)

func TestProducer(t *testing.T) {
	skipWithoutRedis(t)

	client := redmq.NewRedisClient(network, address, password)
	defer client.Close()

	// keep at most ten messages in the stream
	producer := redmq.NewProducer(client, redmq.WithMsgQueueLen(10))

	ctx := context.Background()
	msgID, err := producer.SendMsg(ctx, newTopicName("redmq_example_topic"), "test_kk", "test_vv")
	if err != nil {
		t.Errorf("SendMsg() error = %v", err)
		return
	}
	t.Logf("published msg id: %s", msgID)
}
