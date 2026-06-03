package transport

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/hangtiancheng/lark-go/lark_rpc/internal/codec"
)

type streamFrame struct {
	body []byte
	err  error
}

type ClientStreamConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan streamFrame
	codec  codec.Codec
	once   sync.Once
}

func NewClientStreamConn(ctx context.Context, cc codec.Codec) *ClientStreamConn {
	ctx, cancel := context.WithCancel(ctx)
	return &ClientStreamConn{
		ctx:    ctx,
		cancel: cancel,
		ch:     make(chan streamFrame, 64),
		codec:  cc,
	}
}

func (s *ClientStreamConn) Push(body []byte) {
	select {
	case s.ch <- streamFrame{body: body}:
	case <-s.ctx.Done():
	}
}

func (s *ClientStreamConn) End() {
	s.once.Do(func() {
		select {
		case s.ch <- streamFrame{err: io.EOF}:
		case <-s.ctx.Done():
		}
	})
}

func (s *ClientStreamConn) Error(err error) {
	s.once.Do(func() {
		select {
		case s.ch <- streamFrame{err: err}:
		case <-s.ctx.Done():
		}
	})
}

func (s *ClientStreamConn) Recv(msg interface{}) error {
	select {
	case frame, ok := <-s.ch:
		if !ok {
			return io.EOF
		}
		if frame.err != nil {
			return frame.err
		}
		return s.codec.Unmarshal(frame.body, msg)
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *ClientStreamConn) Context() context.Context {
	return s.ctx
}

func (s *ClientStreamConn) Cancel() {
	s.cancel()
}

var ErrStreamNotFound = errors.New("stream not found")
