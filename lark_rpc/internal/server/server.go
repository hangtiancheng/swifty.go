package server

import (
	"log"
	"net"
	"sync"

	"github.com/hangtiancheng/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark_rpc/internal/limiter"
	"github.com/hangtiancheng/lark_rpc/internal/protocol"
	"github.com/hangtiancheng/lark_rpc/internal/transport"
)

type Server struct {
	addr     string
	services map[string]interface{}
	limiter  *limiter.TokenBucket
	listener net.Listener
	handler  *Handler
	codec    codec.Codec

	conns   map[*transport.TCPConnection]struct{}
	closing chan struct{}
	mu      sync.Mutex
	once    sync.Once
}

func mustNewHandler() *Handler {
	h, err := NewHandler(nil, WithHandlerCodec(codec.JSON))
	if err != nil {
		panic(err)
	}
	return h
}

func NewServer(addr string, opts ...ServerOption) (*Server, error) {
	s := &Server{
		addr:     addr,
		services: make(map[string]interface{}),
		limiter:  limiter.NewTokenBucket(10000),
		handler:  mustNewHandler(),
		conns:    make(map[*transport.TCPConnection]struct{}),
		closing:  make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Server) Register(name string, service interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[name] = service
}

// Handle serves requests serially on one connection.
func (s *Server) Handle(conn *transport.TCPConnection) {
	defer conn.Close()
	for {
		msg, err := conn.Read()
		if err != nil {
			return
		}

		if !s.limiter.Allow() {
			resp := &protocol.Message{
				Header: &protocol.Header{
					RequestID:   msg.Header.RequestID,
					Error:       "rate limit exceeded",
					Compression: codec.CompressionGzip,
				},
			}
			_ = conn.Write(resp)
			continue
		}
		s.mu.Lock()
		service := s.services[msg.Header.ServiceName]
		s.mu.Unlock()
		s.handler.Process(conn, msg, service)
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.closing:
				return nil
			default:
				continue
			}
		}

		tcpConn := transport.NewTCPConnection(conn)

		s.mu.Lock()
		s.conns[tcpConn] = struct{}{}
		s.mu.Unlock()

		go func() {
			s.Handle(tcpConn)
			s.mu.Lock()
			delete(s.conns, tcpConn)
			s.mu.Unlock()
		}()
	}

}

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

func (s *Server) Close() {
	s.Shutdown()
}

func (s *Server) Shutdown() {
	s.once.Do(func() {
		close(s.closing)

		if s.listener != nil {
			_ = s.listener.Close()
		}

		s.limiter.Stop()

		s.mu.Lock()
		defer s.mu.Unlock()
		for conn := range s.conns {
			_ = conn.Close()
		}

		log.Println("server shutdown complete")
	})
}
