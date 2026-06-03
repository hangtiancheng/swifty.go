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

	srv, err := server.NewServer(":9090", server.WithServerCodec(codec.JSON))
	if err != nil {
		log.Println("server.NewServer error ", err.Error())
		return
	}
	srv.Register("Arith", &api.Arith{})
	srv.Register("Arith2", &api.Arith2{})
	err = reg.Register("Arith", registry.Instance{
		Addr: "localhost:9090",
	}, 10)
	if err != nil {
		log.Fatal(err)
	}
	err = reg.Register("Arith2", registry.Instance{
		Addr: "localhost:9090",
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

	log.Println("server started at :9090")
	srv.Start()
}
