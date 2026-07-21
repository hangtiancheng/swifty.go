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

package client

import (
	"context"
	"errors"
	"time"

	"github.com/hangtiancheng/swifty.go/swifty_rpc/internal/codec"
	"github.com/hangtiancheng/swifty.go/swifty_rpc/internal/protocol"
	"github.com/hangtiancheng/swifty.go/swifty_rpc/internal/stream"
	"github.com/hangtiancheng/swifty.go/swifty_rpc/internal/transport"
)

func (c *Client) InvokeAsync(ctx context.Context, service string, method string, args interface{}) (*transport.Future, error) {
	future, err := c.invokeAsync(ctx, service, method, args)
	if err != nil {
		return nil, err
	}
	go func() {
		select {
		case <-future.DoneChan():
		case <-time.After(c.timeout):
			future.Done(nil, context.DeadlineExceeded)
		}
	}()
	return future, nil
}

// Invoke runs one RPC call and applies the client timeout to both send and wait.
func (c *Client) Invoke(ctx context.Context, service string, method string, args interface{}, reply interface{}) error {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	future, err := c.invokeAsync(callCtx, service, method, args)
	if err != nil {
		return err
	}
	return future.GetResultWithContext(callCtx, reply)
}

func (c *Client) InvokeStream(ctx context.Context, service string, method string, args interface{}) (stream.ClientStream, error) {
	if !c.limiter.Allow() {
		return nil, errors.New("rate limit exceeded")
	}

	addr, err := c.getAddr(service)
	if err != nil {
		return nil, err
	}
	br := c.getBreaker(service, addr)

	if !br.Allow() {
		return nil, errors.New("circuit breaker open")
	}

	conn, err := c.getPool(addr).Acquire(ctx)
	if err != nil {
		return nil, err
	}

	body, err := c.codec.Marshal(args)
	if err != nil {
		return nil, err
	}

	msg := &protocol.Message{
		Header: &protocol.Header{
			ServiceName: service,
			MethodName:  method,
			Compression: codec.CompressionGzip,
		},
		Body: body,
	}

	s, err := conn.SendStream(ctx, msg, c.codec)
	if err != nil {
		br.RecordFailure()
		return nil, err
	}
	return s, nil
}

func (c *Client) invokeAsync(ctx context.Context, service string, method string, args interface{}) (*transport.Future, error) {
	if !c.limiter.Allow() {
		return nil, errors.New("rate limit exceeded")
	}

	addr, err := c.getAddr(service)
	if err != nil {
		return nil, err
	}
	br := c.getBreaker(service, addr)

	if !br.Allow() {
		return nil, errors.New("circuit breaker open")
	}

	conn, err := c.getPool(addr).Acquire(ctx)
	if err != nil {
		return nil, err
	}

	body, err := c.codec.Marshal(args)
	if err != nil {
		return nil, err
	}

	req := &protocol.Message{
		Header: &protocol.Header{
			ServiceName: service,
			MethodName:  method,
			Compression: codec.CompressionGzip,
		},
		Body: body,
	}
	future, err := conn.SendAsyncWithCodec(req, c.codec)
	if err != nil {
		br.RecordFailure()
		return nil, err
	}

	future.OnComplete(func(err error) {
		if err != nil {
			br.RecordFailure()
			return
		}
		br.RecordSuccess()
	})
	return future, nil
}
