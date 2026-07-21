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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/constant"
	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/dao"
	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/model"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/util"

	"github.com/hangtiancheng/swifty.go/swifty_http"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	heartbeatInterval = 30 * time.Second
	// Three missed heartbeats mark the connection dead.
	readIdleTimeout = 90 * time.Second
)

type ChatMessageRequest struct {
	SessionId  string `json:"session_id"`
	Type       int8   `json:"type"`
	Content    string `json:"content"`
	Url        string `json:"url"`
	SendId     string `json:"send_id"`
	SendName   string `json:"send_name"`
	SendAvatar string `json:"send_avatar"`
	ReceiveId  string `json:"receive_id"`
	FileType   string `json:"file_type"`
	FileName   string `json:"file_name"`
	FileSize   string `json:"file_size"`
	AVdata     string `json:"av_data"`
}

type Client struct {
	Conn      *swifty_http.WSConn
	Uuid      string
	SendBack  chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

// shutdown closes the connection and signals the write goroutine to exit.
// Channels are never closed, so concurrent senders cannot panic.
func (c *Client) shutdown() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.Conn.Close()
	})
}

type Server struct {
	Clients  map[string]*Client
	mutex    sync.Mutex
	Transmit chan []byte
	done     chan struct{}
	stopOnce sync.Once
}

var ChatServer *Server

func init() {
	ChatServer = &Server{
		Clients:  make(map[string]*Client),
		Transmit: make(chan []byte, constant.ChannelSize),
		done:     make(chan struct{}),
	}
}

func (s *Server) Start() {
	for {
		select {
		case <-s.done:
			s.mutex.Lock()
			clients := make([]*Client, 0, len(s.Clients))
			for _, c := range s.Clients {
				clients = append(clients, c)
			}
			s.Clients = make(map[string]*Client)
			s.mutex.Unlock()
			for _, c := range clients {
				c.shutdown()
			}
			return
		case data := <-s.Transmit:
			s.handleMessage(data)
		}
	}
}

// Stop terminates the event loop and closes every client connection.
func (s *Server) Stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

func (s *Server) register(client *Client) {
	s.mutex.Lock()
	old := s.Clients[client.Uuid]
	s.Clients[client.Uuid] = client
	s.mutex.Unlock()
	if old != nil {
		old.shutdown()
	}
	log.Printf("user %s connected", client.Uuid)
	_ = client.Conn.WriteText("welcome to swifty chat")
}

func (s *Server) unregister(client *Client) {
	s.mutex.Lock()
	if cur, ok := s.Clients[client.Uuid]; ok && cur == client {
		delete(s.Clients, client.Uuid)
		log.Printf("user %s disconnected", client.Uuid)
	}
	s.mutex.Unlock()
	client.shutdown()
}

func (s *Server) handleMessage(data []byte) {
	var req ChatMessageRequest
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("handleMessage: unmarshal failed: %v", err)
		return
	}

	msg := model.Message{
		Uuid:       fmt.Sprintf("M%s", util.GetNowAndLenRandomString(11)),
		SessionId:  req.SessionId,
		Type:       req.Type,
		Content:    req.Content,
		Url:        req.Url,
		SendId:     req.SendId,
		SendName:   req.SendName,
		SendAvatar: req.SendAvatar,
		ReceiveId:  req.ReceiveId,
		FileType:   req.FileType,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		Status:     constant.MessageUnsent,
		CreatedAt:  time.Now(),
		AVdata:     req.AVdata,
	}

	if req.Type == constant.MessageAudioOrVideo {
		// only persist certain AV signals
		type avData struct {
			MessageId string `json:"messageId"`
			Type      string `json:"type"`
		}
		var av avData
		_ = json.Unmarshal([]byte(req.AVdata), &av)
		if av.MessageId == "PROXY" && (av.Type == "start_call" || av.Type == "receive_call" || av.Type == "reject_call") {
			bgCtx := bgCtx()
			if _, err := dao.Engine.Model(&msg).Insert(bgCtx, &msg); err != nil {
				log.Printf("handleMessage: insert AV message failed: %v", err)
			}
		}
		// AV signaling must never echo back to the sender: the caller would
		// see its own start_call as an incoming call.
		s.broadcast(req, msg, false)
		return
	}

	bgCtx := bgCtx()
	if _, err := dao.Engine.Model(&msg).Insert(bgCtx, &msg); err != nil {
		log.Printf("handleMessage: insert message failed: %v", err)
		return
	}

	s.broadcast(req, msg, true)
}

// broadcast delivers the message to its receiver(s). Sends are non-blocking:
// a slow client's full buffer drops the payload instead of stalling the
// event loop while holding the server mutex.
func (s *Server) broadcast(req ChatMessageRequest, msg model.Message, echoToSender bool) {
	if msg.ReceiveId == "" {
		log.Println("broadcast: empty receive_id, skip")
		return
	}
	rsp := MessageListItem{
		Uuid:   msg.Uuid,
		SendId: msg.SendId, SendName: msg.SendName, SendAvatar: req.SendAvatar,
		ReceiveId: msg.ReceiveId, Type: msg.Type, Content: msg.Content,
		Url: msg.Url, FileSize: msg.FileSize, FileName: msg.FileName,
		FileType: msg.FileType, CreatedAt: msg.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if msg.Type == constant.MessageAudioOrVideo {
		rsp.AVdata = msg.AVdata
	}
	jsonMsg, err := json.Marshal(rsp)
	if err != nil {
		log.Printf("broadcast: marshal failed: %v", err)
		return
	}

	var targets []string
	if msg.ReceiveId[0] == 'U' {
		targets = append(targets, msg.ReceiveId)
		if echoToSender && msg.SendId != msg.ReceiveId {
			targets = append(targets, msg.SendId)
		}
	} else if msg.ReceiveId[0] == 'G' {
		var group model.GroupInfo
		if err := dao.ActiveQuery(&group).Where("uuid", msg.ReceiveId).First(bgCtx(), &group); err != nil {
			log.Printf("broadcast: load group %s failed: %v", msg.ReceiveId, err)
			return
		}
		for _, member := range group.Members {
			if !echoToSender && member == msg.SendId {
				continue
			}
			targets = append(targets, member)
		}
	} else {
		return
	}

	s.mutex.Lock()
	clients := make([]*Client, 0, len(targets))
	for _, id := range targets {
		if c, ok := s.Clients[id]; ok {
			clients = append(clients, c)
		}
	}
	s.mutex.Unlock()

	delivered := 0
	for _, c := range clients {
		select {
		case c.SendBack <- jsonMsg:
			delivered++
		default:
			log.Printf("broadcast: client %s buffer full, message %s dropped", c.Uuid, msg.Uuid)
		}
	}
	if delivered > 0 && msg.Type != constant.MessageAudioOrVideo {
		s.markSent(msg.Uuid)
	}
}

func (s *Server) markSent(uuid string) {
	bgCtx := bgCtx()
	if _, err := dao.Engine.Model(&model.Message{}).Where("uuid", uuid).Update(bgCtx, bson.M{
		"status":  constant.MessageSent,
		"send_at": time.Now(),
	}); err != nil {
		log.Printf("markSent %s failed: %v", uuid, err)
	}
}

func NewClientInit(ws *swifty_http.WSConn, clientId string) {
	client := &Client{
		Conn:     ws,
		Uuid:     clientId,
		SendBack: make(chan []byte, constant.ChannelSize),
		done:     make(chan struct{}),
	}
	ChatServer.register(client)
	go clientWrite(client)
	go clientRead(client)
}

func ClientLogout(clientId string) (string, int) {
	ChatServer.mutex.Lock()
	client, ok := ChatServer.Clients[clientId]
	ChatServer.mutex.Unlock()
	if ok {
		ChatServer.unregister(client)
	}
	return "logout successful", 0
}

// clientRead runs the event-driven read loop with heartbeat-based dead
// connection detection: the server pings every heartbeatInterval and the
// read deadline is refreshed on every pong or message.
func clientRead(c *Client) {
	defer ChatServer.unregister(c)

	stopHeartbeat := c.Conn.Heartbeat(heartbeatInterval)
	defer stopHeartbeat()

	refresh := func() { _ = c.Conn.SetReadDeadline(time.Now().Add(readIdleTimeout)) }
	refresh()
	c.Conn.OnPong(func([]byte) { refresh() })
	c.Conn.OnError(func(err error) {
		log.Printf("ws read error for %s: %v", c.Uuid, err)
	})
	c.Conn.OnMessage(func(messageType int, data []byte) {
		refresh()
		if messageType != swifty_http.TextMessage {
			return
		}
		payload := make([]byte, len(data))
		copy(payload, data)
		select {
		case ChatServer.Transmit <- payload:
		default:
			log.Printf("transmit channel full, message from %s rejected", c.Uuid)
			_ = c.Conn.WriteText(`{"type":-1,"send_id":"","receive_id":"","content":"message send failed, please retry"}`)
		}
	})
	c.Conn.Listen()
}

func clientWrite(c *Client) {
	for {
		select {
		case msg := <-c.SendBack:
			if err := c.Conn.WriteMessage(swifty_http.TextMessage, msg); err != nil {
				log.Printf("ws write error for %s: %v", c.Uuid, err)
				ChatServer.unregister(c)
				return
			}
		case <-c.done:
			return
		}
	}
}

func GetOnlineUserList() []string {
	ChatServer.mutex.Lock()
	defer ChatServer.mutex.Unlock()
	users := make([]string, 0, len(ChatServer.Clients))
	for uuid := range ChatServer.Clients {
		users = append(users, uuid)
	}
	return users
}

func bgCtx() context.Context {
	return context.Background()
}
