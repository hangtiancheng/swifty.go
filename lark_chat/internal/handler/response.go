package handler

import (
	"github.com/hangtiancheng/lark-go/lark_http"
)

func JsonBack(ctx *lark_http.Context, message string, ret int, data interface{}) {
	ctx.Status = 200
	switch ret {
	case 0:
		resp := lark_http.H{"code": 200, "message": message}
		if data != nil {
			resp["data"] = data
		}
		ctx.JSON(resp)
	case -2:
		ctx.JSON(lark_http.H{"code": 400, "message": message})
	default:
		ctx.JSON(lark_http.H{"code": 500, "message": message})
	}
}
