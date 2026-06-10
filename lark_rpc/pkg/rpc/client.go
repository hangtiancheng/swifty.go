package rpc

import (
	"context"
	"time"

	internal_client "github.com/hangtiancheng/lark-go/lark_rpc/internal/client"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/load_balance"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/protocol"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/registry"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/transport"
)

type DialOption func(*dialOptions)

type dialOptions struct {
	timeout      time.Duration
	codecType    codec.Type
	registry     *registry.Registry
	loadBalancer load_balance.LoadBalancer
}

func WithTimeout(d time.Duration) DialOption {
	return func(o *dialOptions) { o.timeout = d }
}

func WithDialCodec(t CodecType) DialOption {
	return func(o *dialOptions) { o.codecType = t }
}

func WithRegistry(reg *Registry) DialOption {
	return func(o *dialOptions) { o.registry = reg }
}

func WithLoadBalancer(lb LoadBalancer) DialOption {
	return func(o *dialOptions) { o.loadBalancer = lb }
}

type connMode int

const (
	modeStatic connMode = iota
	modeRegistry
)

type ClientConn struct {
	mode      connMode
	pool      *transport.ConnectionPool
	regClient *internal_client.Client
	codec     codec.Codec
	timeout   time.Duration
}

func Dial(target string, opts ...DialOption) (*ClientConn, error) {
	o := &dialOptions{
		timeout:   5 * time.Second,
		codecType: codec.JSON,
	}
	for _, opt := range opts {
		opt(o)
	}

	cc, err := codec.New(o.codecType)
	if err != nil {
		return nil, err
	}

	if o.registry != nil {
		var clientOpts []internal_client.ClientOption
		clientOpts = append(clientOpts, internal_client.WithClientTimeout(o.timeout))
		clientOpts = append(clientOpts, internal_client.WithClientCodec(o.codecType))
		if o.loadBalancer != nil {
			clientOpts = append(clientOpts, internal_client.WithClientLoadBalancer(o.loadBalancer))
		}
		regClient, err := internal_client.NewClient(o.registry, clientOpts...)
		if err != nil {
			return nil, err
		}
		return &ClientConn{
			mode:      modeRegistry,
			regClient: regClient,
			codec:     cc,
			timeout:   o.timeout,
		}, nil
	}

	pool := transport.NewConnectionPool(target, 0, 1)
	return &ClientConn{
		mode:    modeStatic,
		pool:    pool,
		codec:   cc,
		timeout: o.timeout,
	}, nil
}

func (cc *ClientConn) Invoke(ctx context.Context, service, method string, args, reply interface{}) error {
	switch cc.mode {
	case modeRegistry:
		return cc.regClient.Invoke(ctx, service, method, args, reply)
	default:
		return cc.invokeStatic(ctx, service, method, args, reply)
	}
}

func (cc *ClientConn) invokeStatic(ctx context.Context, service, method string, args, reply interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, cc.timeout)
	defer cancel()

	conn, err := cc.pool.Acquire(ctx)
	if err != nil {
		return err
	}

	body, err := cc.codec.Marshal(args)
	if err != nil {
		return err
	}

	future, err := conn.SendAsyncWithCodec(&protocol.Message{
		Header: &protocol.Header{
			ServiceName: service,
			MethodName:  method,
			Compression: codec.CompressionGzip,
		},
		Body: body,
	}, cc.codec)
	if err != nil {
		return err
	}
	return future.GetResultWithContext(ctx, reply)
}

func (cc *ClientConn) NewStream(ctx context.Context, service, method string, args interface{}) (ClientStream, error) {
	switch cc.mode {
	case modeRegistry:
		return cc.regClient.InvokeStream(ctx, service, method, args)
	default:
		return cc.newStreamStatic(ctx, service, method, args)
	}
}

func (cc *ClientConn) newStreamStatic(ctx context.Context, service, method string, args interface{}) (ClientStream, error) {
	conn, err := cc.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	body, err := cc.codec.Marshal(args)
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

	stream, err := conn.SendStream(ctx, msg, cc.codec)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (cc *ClientConn) Close() error {
	switch cc.mode {
	case modeRegistry:
		cc.regClient.Close()
	default:
		cc.pool.Close()
	}
	return nil
}
