package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hangtiancheng/lark-go/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/protocol"
)

type TCPClient struct {
	conn *TCPConnection
	addr string

	writeMu sync.Mutex
	seq     uint64

	pending sync.Map // map[uint64]*Future
	streams sync.Map // map[uint64]*ClientStreamConn

	closed int32
}

func newTCPClient(addr string) (*TCPClient, error) {
	rawConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	c := &TCPClient{
		conn: NewTCPConnection(rawConn),
		addr: addr,
	}

	go c.readLoop()
	return c, nil
}

func (c *TCPClient) nextSeq() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

func (c *TCPClient) SendAsync(msg *protocol.Message) (*Future, error) {
	return c.SendAsyncWithCodec(msg, nil)
}

func (c *TCPClient) SendAsyncWithCodec(msg *protocol.Message, cc codec.Codec) (*Future, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errors.New("connection closed")
	}

	seq := c.nextSeq()
	msg.Header.RequestID = seq

	future := NewFutureWithCodec(cc)
	c.pending.Store(seq, future)

	c.writeMu.Lock()
	err := c.conn.Write(msg)
	c.writeMu.Unlock()

	if err != nil {
		c.pending.Delete(seq)
		c.fail(err)
		return nil, err
	}

	return future, nil
}

func (c *TCPClient) readLoop() {
	for {
		msg, err := c.conn.Read()
		if err != nil {
			c.fail(err)
			return
		}

		seq := msg.Header.RequestID

		switch msg.Header.StreamFlag {
		case protocol.StreamData:
			if val, ok := c.streams.Load(seq); ok {
				val.(*ClientStreamConn).Push(msg.Body)
			}
		case protocol.StreamEnd:
			if val, ok := c.streams.LoadAndDelete(seq); ok {
				val.(*ClientStreamConn).End()
			}
		case protocol.StreamError:
			if val, ok := c.streams.LoadAndDelete(seq); ok {
				val.(*ClientStreamConn).Error(errors.New(msg.Header.Error))
			}
		default:
			val, ok := c.pending.LoadAndDelete(seq)
			if !ok {
				continue
			}
			future := val.(*Future)
			if msg.Header.Error != "" {
				future.Done(nil, errors.New(msg.Header.Error))
			} else {
				future.Done(msg.Body, nil)
			}
		}
	}
}

func (c *TCPClient) SendStream(ctx context.Context, msg *protocol.Message, cc codec.Codec) (*ClientStreamConn, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errors.New("connection closed")
	}

	seq := c.nextSeq()
	msg.Header.RequestID = seq

	stream := NewClientStreamConn(ctx, cc)
	c.streams.Store(seq, stream)

	c.writeMu.Lock()
	err := c.conn.Write(msg)
	c.writeMu.Unlock()

	if err != nil {
		c.streams.Delete(seq)
		c.fail(err)
		return nil, err
	}

	return stream, nil
}

func (c *TCPClient) fail(err error) {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return
	}

	_ = c.conn.Close()

	c.pending.Range(func(key, value interface{}) bool {
		future := value.(*Future)
		future.Done(nil, err)
		c.pending.Delete(key)
		return true
	})

	c.streams.Range(func(key, value interface{}) bool {
		value.(*ClientStreamConn).Error(err)
		c.streams.Delete(key)
		return true
	})
}

func (c *TCPClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	return c.conn.Close()
}
