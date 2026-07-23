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

package handler

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/log"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/server"
)

type Handler struct {
	sync.Once
	mu     sync.RWMutex
	conns  map[net.Conn]struct{}
	closed atomic.Bool

	db        DB
	parser    Parser
	persister Persister
	logger    log.Logger
}

func NewHandler(db DB, persister Persister, parser Parser, logger log.Logger) (server.Handler, error) {
	h := Handler{
		conns:     make(map[net.Conn]struct{}),
		persister: persister,
		logger:    logger,
		db:        db,
		parser:    parser,
	}

	return &h, nil
}

func (h *Handler) Start() error {
	// Load the persistence file and restore content
	reloader, err := h.persister.Reloader()
	if err != nil {
		return err
	}
	defer reloader.Close()
	h.handle(SetLoadingPattern(context.Background()), newFakeReaderWriter(reloader))
	return nil
}

func (h *Handler) Handle(ctx context.Context, conn net.Conn) {
	h.mu.Lock()
	// Check if the db is already closed
	if h.closed.Load() {
		h.mu.Unlock()
		return
	}

	// Cache the current connection
	h.conns[conn] = struct{}{}
	h.mu.Unlock()

	h.handle(ctx, conn)
}

func (h *Handler) handle(ctx context.Context, conn io.ReadWriter) {
	// Process continuously
	stream := h.parser.ParseStream(conn)
	for {
		select {
		case <-ctx.Done():
			h.logger.Warnf("[handler]handle ctx err: %s", ctx.Err().Error())
			return

		case droplet := <-stream:
			if err := h.handleDroplet(ctx, conn, droplet); err != nil {
				h.logger.Errorf("[handler]conn terminated, err: %s", droplet.Err.Error())
				return
			}
		}
	}
}

func (h *Handler) handleDroplet(ctx context.Context, conn io.ReadWriter, droplet *Droplet) error {
	if droplet.Terminated() {
		return droplet.Err
	}

	if droplet.Err != nil {
		_, _ = conn.Write(droplet.Reply.ToBytes())
		h.logger.Errorf("[handler]conn request, err: %s", droplet.Err.Error())
		return nil
	}

	if droplet.Reply == nil {
		h.logger.Errorf("[handler]conn empty request")
		return nil
	}

	// The request must be a MultiReply
	multiReply, ok := droplet.Reply.(MultiReply)
	if !ok {
		h.logger.Errorf("[handler]conn invalid request: %s", droplet.Reply.ToBytes())
		return nil
	}

	if reply := h.db.Do(ctx, multiReply.Args()); reply != nil {
		_, _ = conn.Write(reply.ToBytes())
		return nil
	}

	_, _ = conn.Write(UnknownErrReplyBytes)
	return nil
}

func (h *Handler) Close() {
	h.Once.Do(func() {
		h.logger.Warnf("[handler]handler closing...")
		h.closed.Store(true)
		h.mu.RLock()
		defer h.mu.RUnlock()
		for conn := range h.conns {
			if err := conn.Close(); err != nil {
				h.logger.Errorf("[handler]close conn err, local addr: %s, err: %s", conn.LocalAddr().String(), err.Error())
			}
		}
		h.conns = nil
		h.db.Close()
		h.persister.Close()
	})
}
