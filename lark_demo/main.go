package main

import (
	"context"
	"log"
	"time"

	rpc "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
	"github.com/hangtiancheng/lark_demo/internal/ai"
	"github.com/hangtiancheng/lark_demo/internal/app"
	"github.com/hangtiancheng/lark_demo/internal/config"
	"github.com/hangtiancheng/lark_demo/internal/rpc_client"
	"github.com/hangtiancheng/lark_demo/internal/service"
	"github.com/hangtiancheng/lark_demo/internal/store"
)

type AIService struct {
	services *service.Services
}

func (s *AIService) Complete(req *rpc_client.AIRequest, reply *rpc_client.AIResponse) error {
	answer, result := s.services.Answer(context.Background(), req.Username, req.SessionID, req.Question, req.ModelType)
	reply.Answer = answer
	reply.Code = int(result)
	return nil
}

func (s *AIService) CompleteStream(req *rpc_client.AIRequest, stream rpc.ServerStream) error {
	return s.services.AnswerStream(stream.Context(), req.Username, req.SessionID, req.Question, req.ModelType, func(chunk string) {
		_ = stream.Send(&rpc_client.AIStreamChunk{Content: chunk})
	})
}

type rpcClient struct {
	client *rpc.StaticClient
}

func (c *rpcClient) Complete(ctx context.Context, req rpc_client.AIRequest) (rpc_client.AIResponse, error) {
	var reply rpc_client.AIResponse
	err := c.client.Invoke(ctx, "AIService", "Complete", &req, &reply)
	return reply, err
}

func (c *rpcClient) CompleteStream(ctx context.Context, req rpc_client.AIRequest) (rpc.ClientStream, error) {
	return c.client.InvokeStream(ctx, "AIService", "CompleteStream", &req)
}

func main() {
	cfg := config.Load()
	st, err := store.Open(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	manager := ai.NewManager(cfg, st)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := st.LoadMessagesInto(ctx, manager); err != nil {
		log.Printf("load messages failed: %v", err)
	}
	services := service.New(cfg, st, manager)
	server, err := rpc.NewServer(cfg.RPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	server.Register("AIService", &AIService{services: services})
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("rpc server stopped: %v", err)
		}
	}()
	client, err := dialRPC(cfg.RPCAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	httpApp := app.New(cfg, services, newCache(), &rpcClient{client: client})
	addr := cfg.AppHost + ":" + cfg.AppPort
	log.Printf("server v2 listening on %s", addr)
	if err := httpApp.Engine().Run(addr); err != nil {
		log.Fatal(err)
	}
}

func dialRPC(addr string) (*rpc.StaticClient, error) {
	var lastErr error
	for i := 0; i < 20; i++ {
		client, err := rpc.NewStaticClient(addr, 30*time.Second)
		if err == nil {
			return client, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return nil, lastErr
}
