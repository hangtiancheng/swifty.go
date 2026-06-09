package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lark_chat/internal/config"
	"lark_chat/internal/dao"
	"lark_chat/internal/router"
	"lark_chat/internal/service"
)

func main() {
	conf := config.Load("config.json")
	dao.InitMongo()
	dao.InitCache()

	go service.ChatServer.Start()

	app := router.Setup()
	addr := fmt.Sprintf("%s:%d", conf.App.Host, conf.App.Port)
	log.Printf("server starting on %s", addr)

	go func() {
		if err := app.Listen(addr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.Shutdown(ctx)
	dao.CloseCache()
	dao.CloseMongo()
	log.Println("server stopped")
}
