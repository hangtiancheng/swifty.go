package handler

import (
	"github.com/hangtiancheng/lark-go/lark_http"
	"lark_chat/internal/service"
)

func GetOnlineUsers(ctx *lark_http.Context, next func()) {
	users := service.GetOnlineUserList()
	JsonBack(ctx, "success", 0, users)
}
