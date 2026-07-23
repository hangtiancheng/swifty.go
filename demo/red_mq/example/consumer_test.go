package example

import (
	"context"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/red_mq"
	"github.com/hangtiancheng/swifty.go/demo/red_mq/redis"
)

const (
	network       = "tcp"
	address       = "请输入 redis 地址"
	password      = "请输入 redis 密码"
	topic         = "请输入 topic 名称"
	consumerGroup = "请输入消费者组名称"
	consumerID    = "请输入消费者名称"
)

// 自定义实现的死信队列
type DemoDeadLetterMailbox struct {
	do func(msg *redis.MsgEntity)
}

func NewDemoDeadLetterMailbox(do func(msg *redis.MsgEntity)) *DemoDeadLetterMailbox {
	return &DemoDeadLetterMailbox{
		do: do,
	}
}

// 死信队列接收消息的处理方法
func (d *DemoDeadLetterMailbox) Deliver(ctx context.Context, msg *redis.MsgEntity) error {
	d.do(msg)
	return nil
}

func Test_Consumer(t *testing.T) {
	client := redis.NewClient(network, address, password)

	// 接收到消息后的处理函数
	callbackFunc := func(ctx context.Context, msg *redis.MsgEntity) error {
		t.Logf("receive msg, msg id: %s, msg key: %s, msg val: %s", msg.MsgID, msg.Key, msg.Val)
		return nil
	}

	// 自定义实现的死信队列
	demoDeadLetterMailbox := NewDemoDeadLetterMailbox(func(msg *redis.MsgEntity) {
		t.Logf("receive dead letter, msg id: %s, msg key: %s, msg val: %s", msg.MsgID, msg.Key, msg.Val)
	})

	// 构造并启动消费者
	consumer, err := red_mq.NewConsumer(client, topic, consumerGroup, consumerID, callbackFunc,
		// 每条消息最多重试 2 次
		red_mq.WithMaxRetryLimit(2),
		// 每轮接收消息的超时时间为 2 s
		red_mq.WithReceiveTimeout(2*time.Second),
		// 注入自定义实现的死信队列
		red_mq.WithDeadLetterMailbox(demoDeadLetterMailbox))
	if err != nil {
		t.Error(err)
		return
	}
	defer consumer.Stop()

	// 十秒后退出单测程序
	<-time.After(10 * time.Second)
}
