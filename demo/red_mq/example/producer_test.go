package example

import (
	"context"
	"testing"

	"github.com/hangtiancheng/swifty.go/demo/red_mq"
	"github.com/hangtiancheng/swifty.go/demo/red_mq/redis"
)

func Test_Producer(t *testing.T) {
	client := redis.NewClient(network, address, password)
	// 最多保留十条消息
	producer := red_mq.NewProducer(client, red_mq.WithMsgQueueLen(10))
	ctx := context.Background()
	msgID, err := producer.SendMsg(ctx, topic, "test_kk", "test_vv")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(msgID)
}
