package stream

import "context"

type ServerStream interface {
	Send(msg interface{}) error
	Context() context.Context
}

type ClientStream interface {
	Recv(msg interface{}) error
	Context() context.Context
}
