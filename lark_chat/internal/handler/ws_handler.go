package handler

import (
	"github.com/hangtiancheng/lark-go/lark_http"
	"lark_chat/internal/service"
)

func WsLogin(ctx *lark_http.Context, next func()) {
	clientId := ctx.Query("client_id")
	if clientId == "" {
		JsonBack(ctx, "client_id is required", -2, nil)
		return
	}
	ws, err := ctx.Upgrade(&lark_http.UpgradeOptions{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
	})
	if err != nil {
		return
	}
	service.NewClientInit(ws, clientId)
}

func WsLogout(ctx *lark_http.Context, next func()) {
	var req struct {
		OwnerId string `json:"owner_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.ClientLogout(req.OwnerId)
	JsonBack(ctx, msg, ret, nil)
}
