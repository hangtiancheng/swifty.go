package dao

import (
	"context"
	"log"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/config"

	"github.com/hangtiancheng/swifty.go/swifty_orm"
)

var Engine *swifty_orm.Engine

func InitMongo() {
	conf := config.Get()
	var err error
	Engine, err = swifty_orm.NewEngine(context.Background(), conf.Mongo.URI, conf.Mongo.Database)
	if err != nil {
		log.Fatalf("failed to connect mongo: %v", err)
	}
	log.Printf("connected to mongodb: %s/%s", conf.Mongo.URI, conf.Mongo.Database)
}

func CloseMongo() {
	if Engine != nil {
		_ = Engine.Close(context.Background())
	}
}
