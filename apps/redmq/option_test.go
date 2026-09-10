package redmq

import (
	"reflect"
	"testing"
	"time"
)

func Test_repairProducer(t *testing.T) {
	tests := []struct {
		name string
		in   ProducerOptions
		want ProducerOptions
	}{
		{
			name: "zero value falls back to default",
			in:   ProducerOptions{msgQueueLen: 0},
			want: ProducerOptions{msgQueueLen: 500},
		},
		{
			name: "negative value falls back to default",
			in:   ProducerOptions{msgQueueLen: -1},
			want: ProducerOptions{msgQueueLen: 500},
		},
		{
			name: "valid value is kept",
			in:   ProducerOptions{msgQueueLen: 10},
			want: ProducerOptions{msgQueueLen: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.in
			repairProducer(&opts)
			if !reflect.DeepEqual(opts, tt.want) {
				t.Errorf("repairProducer() = %+v, want %+v", opts, tt.want)
			}
		})
	}
}

func Test_repairConsumer(t *testing.T) {
	tests := []struct {
		name  string
		in    ConsumerOptions
		wants func(t *testing.T, got ConsumerOptions)
	}{
		{
			name: "zero value falls back to defaults",
			in:   ConsumerOptions{},
			wants: func(t *testing.T, got ConsumerOptions) {
				t.Helper()
				want := ConsumerOptions{
					receiveTimeout:           2 * time.Second,
					maxRetryLimit:            3,
					deadLetterMailbox:        got.deadLetterMailbox,
					deadLetterDeliverTimeout: time.Second,
					handleMsgsTimeout:        time.Second,
				}
				if got.deadLetterMailbox == nil {
					t.Error("repairConsumer() deadLetterMailbox = nil, want default mailbox")
				}
				if _, ok := got.deadLetterMailbox.(*DeadLetterLogger); !ok {
					t.Errorf("repairConsumer() deadLetterMailbox = %T, want *DeadLetterLogger", got.deadLetterMailbox)
				}
				got.deadLetterMailbox = want.deadLetterMailbox
				if !reflect.DeepEqual(got, want) {
					t.Errorf("repairConsumer() = %+v, want %+v", got, want)
				}
			},
		},
		{
			name: "valid values are kept",
			in: ConsumerOptions{
				receiveTimeout:           3 * time.Second,
				maxRetryLimit:            5,
				deadLetterMailbox:        NewDeadLetterLogger(),
				deadLetterDeliverTimeout: 2 * time.Second,
				handleMsgsTimeout:        4 * time.Second,
			},
			wants: func(t *testing.T, got ConsumerOptions) {
				t.Helper()
				if !reflect.DeepEqual(got, ConsumerOptions{
					receiveTimeout:           3 * time.Second,
					maxRetryLimit:            5,
					deadLetterMailbox:        NewDeadLetterLogger(),
					deadLetterDeliverTimeout: 2 * time.Second,
					handleMsgsTimeout:        4 * time.Second,
				}) {
					t.Errorf("repairConsumer() = %+v, want the input values unchanged", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.in
			repairConsumer(&opts)
			tt.wants(t, opts)
		})
	}
}

func Test_consumerOptions(t *testing.T) {
	mailbox := NewDeadLetterLogger()

	opts := &ConsumerOptions{}
	WithReceiveTimeout(5 * time.Second)(opts)
	WithMaxRetryLimit(7)(opts)
	WithDeadLetterMailbox(mailbox)(opts)
	WithDeadLetterDeliverTimeout(8 * time.Second)(opts)
	WithHandleMsgsTimeout(9 * time.Second)(opts)

	want := ConsumerOptions{
		receiveTimeout:           5 * time.Second,
		maxRetryLimit:            7,
		deadLetterMailbox:        mailbox,
		deadLetterDeliverTimeout: 8 * time.Second,
		handleMsgsTimeout:        9 * time.Second,
	}
	if !reflect.DeepEqual(*opts, want) {
		t.Errorf("consumer options = %+v, want %+v", *opts, want)
	}
}

func Test_producerOptions(t *testing.T) {
	opts := &ProducerOptions{}
	WithMsgQueueLen(10)(opts)

	if !reflect.DeepEqual(*opts, ProducerOptions{msgQueueLen: 10}) {
		t.Errorf("producer options = %+v, want %+v", *opts, ProducerOptions{msgQueueLen: 10})
	}
}
