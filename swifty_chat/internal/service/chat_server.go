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
	Conn     *swifty_http.WSConn
	Uuid     string
	SendTo   chan []byte
	SendBack chan []byte
}

type Server struct {
	Clients  map[string]*Client
	mutex    sync.Mutex
	Transmit chan []byte
	Login    chan *Client
	Logout   chan *Client
}

var ChatServer *Server

func init() {
	ChatServer = &Server{
		Clients:  make(map[string]*Client),
		Transmit: make(chan []byte, constant.ChannelSize),
		Login:    make(chan *Client, constant.ChannelSize),
		Logout:   make(chan *Client, constant.ChannelSize),
	}
}

func (s *Server) Start() {
	for {
		select {
		case client := <-s.Login:
			s.mutex.Lock()
			s.Clients[client.Uuid] = client
			s.mutex.Unlock()
			log.Printf("user %s connected", client.Uuid)
			_ = client.Conn.WriteText("welcome to swifty chat")

		case client := <-s.Logout:
			s.mutex.Lock()
			delete(s.Clients, client.Uuid)
			s.mutex.Unlock()
			log.Printf("user %s disconnected", client.Uuid)

		case data := <-s.Transmit:
			s.handleMessage(data)
		}
	}
}

func (s *Server) handleMessage(data []byte) {
	var req ChatMessageRequest
	if err := json.Unmarshal(data, &req); err != nil {
		log.Println(err)
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
		if av.MessageId != "PROXY" || (av.Type != "start_call" && av.Type != "receive_call" && av.Type != "reject_call") {
			s.broadcastToReceiver(req, msg)
			return
		}
	}

	bgCtx := bgCtx()
	if _, err := dao.Engine.Model(&msg).Insert(bgCtx, &msg); err != nil {
		log.Println(err)
		return
	}

	s.broadcastToReceiver(req, msg)
}

func (s *Server) broadcastToReceiver(req ChatMessageRequest, msg model.Message) {
	if msg.ReceiveId == "" {
		log.Println("broadcastToReceiver: empty receive_id, skip")
		return
	}
	rsp := MessageListItem{
		SendId: msg.SendId, SendName: msg.SendName, SendAvatar: req.SendAvatar,
		ReceiveId: msg.ReceiveId, Type: msg.Type, Content: msg.Content,
		Url: msg.Url, FileSize: msg.FileSize, FileName: msg.FileName,
		FileType: msg.FileType, CreatedAt: msg.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	jsonMsg, _ := json.Marshal(rsp)

	if msg.ReceiveId[0] == 'U' {
		s.mutex.Lock()
		if c, ok := s.Clients[msg.ReceiveId]; ok {
			c.SendBack <- jsonMsg
		}
		if c, ok := s.Clients[msg.SendId]; ok {
			c.SendBack <- jsonMsg
		}
		s.mutex.Unlock()
		s.markSent(msg.Uuid)
	} else if msg.ReceiveId[0] == 'G' {
		var group model.GroupInfo
		bgCtx := bgCtx()
		if err := dao.ActiveQuery(&group).Where("uuid", msg.ReceiveId).First(bgCtx, &group); err != nil {
			log.Println(err)
			return
		}
		s.mutex.Lock()
		for _, member := range group.Members {
			if c, ok := s.Clients[member]; ok {
				c.SendBack <- jsonMsg
			}
		}
		s.mutex.Unlock()
		s.markSent(msg.Uuid)
	}
}

func (s *Server) markSent(uuid string) {
	bgCtx := bgCtx()
	_, _ = dao.Engine.Model(&model.Message{}).Where("uuid", uuid).Update(bgCtx, bson.M{"status": constant.MessageSent})
}

func NewClientInit(ws *swifty_http.WSConn, clientId string) {
	client := &Client{
		Conn:     ws,
		Uuid:     clientId,
		SendTo:   make(chan []byte, constant.ChannelSize),
		SendBack: make(chan []byte, constant.ChannelSize),
	}
	ChatServer.Login <- client
	go clientRead(client)
	go clientWrite(client)
}

func ClientLogout(clientId string) (string, int) {
	ChatServer.mutex.Lock()
	client, ok := ChatServer.Clients[clientId]
	ChatServer.mutex.Unlock()
	if ok {
		ChatServer.Logout <- client
		_ = client.Conn.Close()
		close(client.SendTo)
		close(client.SendBack)
	}
	return "logout successful", 0
}

func clientRead(c *Client) {
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			log.Printf("ws read error for %s: %v", c.Uuid, err)
			ChatServer.mutex.Lock()
			delete(ChatServer.Clients, c.Uuid)
			ChatServer.mutex.Unlock()
			return
		}
		for len(ChatServer.Transmit) < constant.ChannelSize && len(c.SendTo) > 0 {
			ChatServer.Transmit <- <-c.SendTo
		}
		if len(ChatServer.Transmit) < constant.ChannelSize {
			ChatServer.Transmit <- data
		} else if len(c.SendTo) < constant.ChannelSize {
			c.SendTo <- data
		}
	}
}

func clientWrite(c *Client) {
	for msg := range c.SendBack {
		if err := c.Conn.WriteMessage(swifty_http.TextMessage, msg); err != nil {
			log.Printf("ws write error for %s: %v", c.Uuid, err)
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
