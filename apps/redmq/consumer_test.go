package redmq

import (
	"context"
	"errors"
	"testing"

	"github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"
)

func TestNewConsumer_param_validation(t *testing.T) {
	client := NewRedisClient("tcp", "127.0.0.1:6379", "")
	callback := func(ctx context.Context, msg *redis.MsgEntity) error { return nil }

	tests := []struct {
		name         string
		client       *redis.Client
		topic        string
		groupID      string
		consumerID   string
		callbackFunc MsgCallback
		want         error
	}{
		{
			name:         "nil callback",
			client:       client,
			topic:        "topic",
			groupID:      "group",
			consumerID:   "consumer",
			callbackFunc: nil,
			want:         errNilCallback,
		},
		{
			name:         "nil redis client",
			client:       nil,
			topic:        "topic",
			groupID:      "group",
			consumerID:   "consumer",
			callbackFunc: callback,
			want:         errNilRedisClient,
		},
		{
			name:         "empty topic",
			client:       client,
			topic:        "",
			groupID:      "group",
			consumerID:   "consumer",
			callbackFunc: callback,
			want:         errEmptyStreamIDs,
		},
		{
			name:         "empty group id",
			client:       client,
			topic:        "topic",
			groupID:      "",
			consumerID:   "consumer",
			callbackFunc: callback,
			want:         errEmptyStreamIDs,
		},
		{
			name:         "empty consumer id",
			client:       client,
			topic:        "topic",
			groupID:      "group",
			consumerID:   "",
			callbackFunc: callback,
			want:         errEmptyStreamIDs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer, err := NewConsumer(tt.client, tt.topic, tt.groupID, tt.consumerID, tt.callbackFunc)
			if consumer != nil {
				defer consumer.Stop()
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("NewConsumer() error = %v, want %v", err, tt.want)
			}
		})
	}
}
