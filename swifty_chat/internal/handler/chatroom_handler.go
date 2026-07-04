package handler

import (
	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/service"

	"github.com/hangtiancheng/swifty.go/swifty_http"
)

func GetOnlineUsers(ctx *swifty_http.Context, next func()) {
	users := service.GetOnlineUserList()
	JsonBack(ctx, "success", 0, users)
}
