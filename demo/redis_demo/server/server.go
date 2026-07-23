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

package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib/pool"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/log"
)

// Handler processes tcp connections.
type Handler interface {
	Start() error // start the handler
	// Handle each incoming tcp connection.
	Handle(ctx context.Context, conn net.Conn)
	// Close the handler.
	Close()
}

type Server struct {
	runOnce  sync.Once
	stopOnce sync.Once
	handler  Handler
	logger   log.Logger
	stopChan chan struct{}
}

func NewServer(handler Handler, logger log.Logger) *Server {
	return &Server{
		handler:  handler,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

func (s *Server) Serve(address string) error {
	if err := s.handler.Start(); err != nil {
		return err
	}
	var _err error
	s.runOnce.Do(func() {
		// Listen for process signals.
		exitWords := []os.Signal{syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, exitWords...)
		closeChan := make(chan struct{}, 4)
		pool.Submit(func() {
			for {
				select {
				case signal := <-sigChan:
					switch signal {
					case exitWords[0], exitWords[1], exitWords[2], exitWords[3]:
						closeChan <- struct{}{}
						return
					default:
					}
				case <-s.stopChan:
					closeChan <- struct{}{}
					return
				}
			}
		})

		listener, err := net.Listen("tcp", address)
		if err != nil {
			_err = err
			return
		}

		s.listenAndServe(listener, closeChan)
	})

	return _err
}

func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
}

func (s *Server) listenAndServe(listener net.Listener, closeChan chan struct{}) {
	errChan := make(chan error, 1)
	defer close(errChan)

	// On unexpected error, terminate.
	ctx, cancel := context.WithCancel(context.Background())
	pool.Submit(
		func() {
			select {
			case <-closeChan:
				s.logger.Errorf("[server] server closing...")
			case err := <-errChan:
				s.logger.Errorf("[server] server err: %s", err.Error())
			}
			cancel()
			s.logger.Warnf("[server] server closing...")
			s.handler.Close()
			if err := listener.Close(); err != nil {
				s.logger.Errorf("[server] server close listener err: %s", err.Error())
			}
		})

	s.logger.Warnf("[server] server starting...")
	var wg sync.WaitGroup
	// I/O multiplexing: one goroutine per connection.
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Ignore timeout errors.
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				time.Sleep(5 * time.Millisecond)
				continue
			}

			// Unexpected error: stop.
			errChan <- err
			break
		}

		// Allocate a goroutine for each incoming connection.
		wg.Add(1)
		pool.Submit(func() {
			defer wg.Done()
			s.handler.Handle(ctx, conn)
		})
	}

	// Use WaitGroup for graceful shutdown.
	wg.Wait()
}
