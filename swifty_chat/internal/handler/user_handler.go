package handler

import (
	"go.mongodb.org/mongo-driver/bson"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/service"

	"github.com/hangtiancheng/swifty.go/swifty_http"
)

func Login(ctx *swifty_http.Context, next func()) {
	var req struct {
		Telephone string `json:"telephone"`
		Password  string `json:"password"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.Login(ctx.Request.Context(), req.Telephone, req.Password)
	JsonBack(ctx, msg, ret, data)
}

func Register(ctx *swifty_http.Context, next func()) {
	var req struct {
		Telephone string `json:"telephone"`
		Password  string `json:"password"`
		Nickname  string `json:"nickname"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.Register(ctx.Request.Context(), req.Telephone, req.Password, req.Nickname)
	JsonBack(ctx, msg, ret, data)
}

func UpdateUserInfo(ctx *swifty_http.Context, next func()) {
	var req struct {
		Uuid      string `json:"uuid"`
		Nickname  string `json:"nickname"`
		Email     string `json:"email"`
		Birthday  string `json:"birthday"`
		Signature string `json:"signature"`
		Avatar    string `json:"avatar"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	fields := bson.M{}
	if req.Nickname != "" {
		fields["nickname"] = req.Nickname
	}
	if req.Email != "" {
		fields["email"] = req.Email
	}
	if req.Birthday != "" {
		fields["birthday"] = req.Birthday
	}
	if req.Signature != "" {
		fields["signature"] = req.Signature
	}
	if req.Avatar != "" {
		fields["avatar"] = req.Avatar
	}
	msg, ret := service.UpdateUserInfo(ctx.Request.Context(), req.Uuid, fields)
	JsonBack(ctx, msg, ret, nil)
}

func GetUserInfo(ctx *swifty_http.Context, next func()) {
	var req struct {
		Uuid string `json:"uuid"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetUserInfo(ctx.Request.Context(), req.Uuid)
	JsonBack(ctx, msg, ret, data)
}

func GetUserInfoList(ctx *swifty_http.Context, next func()) {
	var req struct {
		OwnerId string `json:"owner_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetUserInfoList(ctx.Request.Context(), req.OwnerId)
	JsonBack(ctx, msg, ret, data)
}

func AbleUsers(ctx *swifty_http.Context, next func()) {
	var req struct {
		UuidList []string `json:"uuid_list"`
		IsAdmin  int8     `json:"is_admin"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.AbleUsers(ctx.Request.Context(), req.UuidList)
	JsonBack(ctx, msg, ret, nil)
}

func DisableUsers(ctx *swifty_http.Context, next func()) {
	var req struct {
		UuidList []string `json:"uuid_list"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.DisableUsers(ctx.Request.Context(), req.UuidList)
	JsonBack(ctx, msg, ret, nil)
}

func DeleteUsers(ctx *swifty_http.Context, next func()) {
	var req struct {
		UuidList []string `json:"uuid_list"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.DeleteUsers(ctx.Request.Context(), req.UuidList)
	JsonBack(ctx, msg, ret, nil)
}

func SetAdmin(ctx *swifty_http.Context, next func()) {
	var req struct {
		UuidList []string `json:"uuid_list"`
		IsAdmin  int8     `json:"is_admin"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.SetAdmin(ctx.Request.Context(), req.UuidList, req.IsAdmin)
	JsonBack(ctx, msg, ret, nil)
}
