package lark_http

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

const (
	wsGUID         = "258EAFA5-E914-47DA-95CA-5BBF24F4B947"
	maxFrameSize   = 65536
	closeNormal    = 1000
	closeGoingAway = 1001
)

var (
	ErrWSClosed       = errors.New("websocket: connection closed")
	ErrWSInvalidFrame = errors.New("websocket: invalid frame")
)

type UpgradeOptions struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
	Subprotocols    []string
}

type WSConn struct {
	conn    net.Conn
	br      *bufio.Reader
	bw      *bufio.Writer
	writeMu sync.Mutex
	readMu  sync.Mutex
	closed  chan struct{}
	once    sync.Once
	onClose func(code int, text string)
	onError func(err error)
}

func (ctx *Context) Upgrade(opts *UpgradeOptions) (*WSConn, error) {
	if opts != nil && opts.CheckOrigin != nil {
		if !opts.CheckOrigin(ctx.Request) {
			ctx.Throw(http.StatusForbidden, "origin not allowed")
			return nil, errors.New("websocket: origin check failed")
		}
	}

	if !headerContains(ctx.Request.Header, "Connection", "upgrade") {
		ctx.Throw(http.StatusBadRequest, "missing Connection: upgrade")
		return nil, errors.New("websocket: missing Connection upgrade")
	}
	if !headerContains(ctx.Request.Header, "Upgrade", "websocket") {
		ctx.Throw(http.StatusBadRequest, "missing Upgrade: websocket")
		return nil, errors.New("websocket: missing Upgrade header")
	}
	key := ctx.Request.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		ctx.Throw(http.StatusBadRequest, "missing Sec-WebSocket-Key")
		return nil, errors.New("websocket: missing key")
	}

	var subprotocol string
	if opts != nil && len(opts.Subprotocols) > 0 {
		clientProtos := ctx.Request.Header.Get("Sec-WebSocket-Protocol")
		subprotocol = negotiateSubprotocol(clientProtos, opts.Subprotocols)
	}

	acceptKey := computeAcceptKey(key)

	hijacker, ok := ctx.Writer.(http.Hijacker)
	if !ok {
		ctx.Throw(http.StatusInternalServerError, "hijack not supported")
		return nil, errors.New("websocket: hijack not supported")
	}

	conn, rwBuf, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	ctx.flushed = true

	var resp strings.Builder
	resp.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	resp.WriteString("Upgrade: websocket\r\n")
	resp.WriteString("Connection: Upgrade\r\n")
	resp.WriteString("Sec-WebSocket-Accept: " + acceptKey + "\r\n")
	if subprotocol != "" {
		resp.WriteString("Sec-WebSocket-Protocol: " + subprotocol + "\r\n")
	}
	resp.WriteString("\r\n")

	if _, err := rwBuf.WriteString(resp.String()); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rwBuf.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	readBufSize := 4096
	writeBufSize := 4096
	if opts != nil {
		if opts.ReadBufferSize > 0 {
			readBufSize = opts.ReadBufferSize
		}
		if opts.WriteBufferSize > 0 {
			writeBufSize = opts.WriteBufferSize
		}
	}

	ws := &WSConn{
		conn:   conn,
		br:     bufio.NewReaderSize(conn, readBufSize),
		bw:     bufio.NewWriterSize(conn, writeBufSize),
		closed: make(chan struct{}),
	}

	return ws, nil
}

func (ws *WSConn) ReadMessage() (messageType int, data []byte, err error) {
	ws.readMu.Lock()
	defer ws.readMu.Unlock()

	for {
		opcode, payload, err := ws.readFrame()
		if err != nil {
			ws.handleError(err)
			return 0, nil, err
		}

		switch opcode {
		case PingMessage:
			_ = ws.writeFrame(PongMessage, payload)
			continue
		case PongMessage:
			continue
		case CloseMessage:
			code, text := parseClosePayload(payload)
			_ = ws.writeCloseFrame(code)
			ws.once.Do(func() { close(ws.closed) })
			if ws.onClose != nil {
				ws.onClose(code, text)
			}
			return 0, nil, ErrWSClosed
		case TextMessage, BinaryMessage:
			return opcode, payload, nil
		default:
			return 0, nil, ErrWSInvalidFrame
		}
	}
}

func (ws *WSConn) ReadJSON(v interface{}) error {
	_, data, err := ws.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (ws *WSConn) WriteMessage(messageType int, data []byte) error {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	return ws.writeFrame(messageType, data)
}

func (ws *WSConn) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ws.WriteMessage(TextMessage, data)
}

func (ws *WSConn) WriteText(text string) error {
	return ws.WriteMessage(TextMessage, []byte(text))
}

func (ws *WSConn) WriteBinary(data []byte) error {
	return ws.WriteMessage(BinaryMessage, data)
}

func (ws *WSConn) Ping() error {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	return ws.writeFrame(PingMessage, nil)
}

func (ws *WSConn) Close() error {
	ws.once.Do(func() { close(ws.closed) })
	ws.writeMu.Lock()
	_ = ws.writeFrame(CloseMessage, formatClosePayload(closeNormal, ""))
	ws.writeMu.Unlock()
	return ws.conn.Close()
}

func (ws *WSConn) CloseWithMessage(code int, text string) error {
	ws.once.Do(func() { close(ws.closed) })
	ws.writeMu.Lock()
	_ = ws.writeFrame(CloseMessage, formatClosePayload(code, text))
	ws.writeMu.Unlock()
	return ws.conn.Close()
}

func (ws *WSConn) Closed() <-chan struct{} {
	return ws.closed
}

func (ws *WSConn) OnClose(fn func(code int, text string)) {
	ws.onClose = fn
}

func (ws *WSConn) OnError(fn func(err error)) {
	ws.onError = fn
}

func (ws *WSConn) SetReadDeadline(t time.Time) error {
	return ws.conn.SetReadDeadline(t)
}

func (ws *WSConn) SetWriteDeadline(t time.Time) error {
	return ws.conn.SetWriteDeadline(t)
}

func (ws *WSConn) Heartbeat(interval time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ws.Ping(); err != nil {
					return
				}
			case <-ws.closed:
				return
			case <-stop:
				return
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

func (ws *WSConn) NetConn() net.Conn {
	return ws.conn
}

// --- frame I/O ---

func (ws *WSConn) readFrame() (opcode int, payload []byte, err error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(ws.br, header); err != nil {
		return 0, nil, err
	}

	// fin := header[0]&0x80 != 0
	opcode = int(header[0] & 0x0F)
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)

	switch {
	case length == 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(ws.br, ext); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case length == 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(ws.br, ext); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(ext)
	}

	if length > maxFrameSize {
		return 0, nil, ErrWSInvalidFrame
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(ws.br, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload = make([]byte, length)
	if _, err := io.ReadFull(ws.br, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}

func (ws *WSConn) writeFrame(opcode int, payload []byte) error {
	var header []byte
	length := len(payload)

	switch {
	case length <= 125:
		header = []byte{byte(0x80 | opcode), byte(length)}
	case length <= 65535:
		header = make([]byte, 4)
		header[0] = byte(0x80 | opcode)
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(length))
	default:
		header = make([]byte, 10)
		header[0] = byte(0x80 | opcode)
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(length))
	}

	if _, err := ws.bw.Write(header); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := ws.bw.Write(payload); err != nil {
			return err
		}
	}
	return ws.bw.Flush()
}

func (ws *WSConn) writeCloseFrame(code int) error {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	return ws.writeFrame(CloseMessage, formatClosePayload(code, ""))
}

func (ws *WSConn) handleError(err error) {
	if ws.onError != nil {
		ws.onError(err)
	}
	ws.once.Do(func() { close(ws.closed) })
}

// --- helpers ---

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func headerContains(h http.Header, key string, value string) bool {
	for _, v := range h[http.CanonicalHeaderKey(key)] {
		for _, s := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(s), value) {
				return true
			}
		}
	}
	return false
}

func negotiateSubprotocol(clientHeader string, serverProtocols []string) string {
	for _, sp := range serverProtocols {
		for _, cp := range strings.Split(clientHeader, ",") {
			if strings.TrimSpace(cp) == sp {
				return sp
			}
		}
	}
	return ""
}

func parseClosePayload(payload []byte) (int, string) {
	if len(payload) < 2 {
		return closeNormal, ""
	}
	code := int(binary.BigEndian.Uint16(payload[:2]))
	text := ""
	if len(payload) > 2 {
		text = string(payload[2:])
		if !utf8.ValidString(text) {
			text = ""
		}
	}
	return code, text
}

func formatClosePayload(code int, text string) []byte {
	payload := make([]byte, 2+len(text))
	binary.BigEndian.PutUint16(payload, uint16(code))
	copy(payload[2:], text)
	return payload
}
