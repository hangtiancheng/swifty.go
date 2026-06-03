package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/hangtiancheng/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark_rpc/internal/registry"
	"github.com/hangtiancheng/lark_rpc/internal/server"
	"github.com/hangtiancheng/lark_rpc/pkg/api"
)

func main() {
	reg, err := registry.NewRegistry([]string{"localhost:2379"})
	if err != nil {
		log.Fatal(err)
	}

	srv, err := server.NewServer(":9091", server.WithServerCodec(codec.JSON))
	if err != nil {
		log.Println("server.NewServer error ", err.Error())
		return
	}
	srv.Register("Arith", &api.Arith{})

	err = reg.Register("Arith", registry.Instance{
		Addr: "localhost:9091",
	}, 10)
	if err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh
		log.Println("graceful shutdown...")
		srv.Shutdown()
	}()

	log.Println("server started at :9091")
	srv.Start()
}
