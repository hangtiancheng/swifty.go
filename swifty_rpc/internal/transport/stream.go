// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package transport

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/hangtiancheng/swifty.go/swifty_rpc/internal/codec"
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
