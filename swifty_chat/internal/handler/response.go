package handler

import (
	"github.com/hangtiancheng/swifty.go/swifty_http"
)

func JsonBack(ctx *swifty_http.Context, message string, ret int, data interface{}) {
	ctx.Status = 200
	switch ret {
	case 0:
		resp := swifty_http.H{"code": 200, "message": message}
		if data != nil {
			resp["data"] = data
		}
		ctx.JSON(resp)
	case -2:
		ctx.JSON(swifty_http.H{"code": 400, "message": message})
	default:
		ctx.JSON(swifty_http.H{"code": 500, "message": message})
	}
}
