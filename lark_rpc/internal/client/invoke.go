package client

import (
	"context"
	"errors"
	"time"

	"github.com/hangtiancheng/lark-go/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/protocol"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/stream"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/transport"
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
