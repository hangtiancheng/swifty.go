package rpc

import (
	"context"
	"time"

	internal_client "github.com/hangtiancheng/lark_rpc/internal/client"
	"github.com/hangtiancheng/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark_rpc/internal/load_balance"
	"github.com/hangtiancheng/lark_rpc/internal/protocol"
	"github.com/hangtiancheng/lark_rpc/internal/registry"
	internal_server "github.com/hangtiancheng/lark_rpc/internal/server"
	"github.com/hangtiancheng/lark_rpc/internal/transport"
)

type Server struct {
	inner *internal_server.Server
}

func NewServer(addr string) (*Server, error) {
	inner, err := internal_server.NewServer(addr)
	if err != nil {
		return nil, err
	}
	return &Server{inner: inner}, nil
}

func (s *Server) Register(name string, service interface{}) {
	s.inner.Register(name, service)
}

func (s *Server) Start() error {
	return s.inner.Start()
}

func (s *Server) Addr() string {
	return s.inner.Addr()
}

func (s *Server) Shutdown() {
	s.inner.Shutdown()
}

type StaticClient struct {
	pool    *transport.ConnectionPool
	codec   codec.Codec
	timeout time.Duration
}

func NewStaticClient(addr string, timeout time.Duration) (*StaticClient, error) {
	cc, err := codec.New(codec.JSON)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &StaticClient{pool: transport.NewConnectionPool(addr, 0, 1), codec: cc, timeout: timeout}, nil
}

func (c *StaticClient) Invoke(ctx context.Context, service string, method string, args interface{}, reply interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	body, err := c.codec.Marshal(args)
	if err != nil {
		return err
	}
	future, err := conn.SendAsyncWithCodec(&protocol.Message{Header: &protocol.Header{ServiceName: service, MethodName: method, Compression: codec.CompressionGzip}, Body: body}, c.codec)
	if err != nil {
		return err
	}
	return future.GetResultWithContext(ctx, reply)
}

func (c *StaticClient) InvokeStream(ctx context.Context, service string, method string, args interface{}) (ClientStream, error) {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	body, err := c.codec.Marshal(args)
	if err != nil {
		return nil, err
	}
	msg := &protocol.Message{
		Header: &protocol.Header{ServiceName: service, MethodName: method, Compression: codec.CompressionGzip},
		Body:   body,
	}
	stream, err := conn.SendStream(ctx, msg, c.codec)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (c *StaticClient) Close() {
	c.pool.Close()
}

type RegistryClient = internal_client.Client
type Registry = registry.Registry
type Instance = registry.Instance
type LoadBalancer = load_balance.LoadBalancer

func NewRegistry(endpoints []string) (*registry.Registry, error) {
	return registry.NewRegistry(endpoints)
}

func NewClient(reg *registry.Registry, opts ...internal_client.ClientOption) (*internal_client.Client, error) {
	return internal_client.NewClient(reg, opts...)
}
