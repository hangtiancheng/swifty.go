package handler

import (
	"github.com/hangtiancheng/lark-go/lark_http"
	"lark_chat/internal/service"
)

func GetUserList(ctx *lark_http.Context, next func()) {
	var req struct {
		OwnerId string `json:"owner_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetUserList(ctx.Request.Context(), req.OwnerId)
	JsonBack(ctx, msg, ret, data)
}

func LoadMyJoinedGroup(ctx *lark_http.Context, next func()) {
	var req struct {
		OwnerId string `json:"owner_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.LoadMyJoinedGroup(ctx.Request.Context(), req.OwnerId)
	JsonBack(ctx, msg, ret, data)
}

func GetContactInfo(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId    string `json:"user_id"`
		ContactId string `json:"contact_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetContactInfo(ctx.Request.Context(), req.UserId, req.ContactId)
	JsonBack(ctx, msg, ret, data)
}

func ApplyContact(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId      string `json:"user_id"`
		ContactId   string `json:"contact_id"`
		ContactType int8   `json:"contact_type"`
		Message     string `json:"message"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.ApplyContact(ctx.Request.Context(), req.UserId, req.ContactId, req.ContactType, req.Message)
	JsonBack(ctx, msg, ret, nil)
}

func GetNewContactList(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId string `json:"user_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetNewContactList(ctx.Request.Context(), req.UserId)
	JsonBack(ctx, msg, ret, data)
}

func PassContactApply(ctx *lark_http.Context, next func()) {
	var req struct {
		ApplyId string `json:"apply_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.PassContactApply(ctx.Request.Context(), req.ApplyId)
	JsonBack(ctx, msg, ret, nil)
}

func BlackContact(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId    string `json:"user_id"`
		ContactId string `json:"contact_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.BlackContact(ctx.Request.Context(), req.UserId, req.ContactId)
	JsonBack(ctx, msg, ret, nil)
}

func CancelBlackContact(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId    string `json:"user_id"`
		ContactId string `json:"contact_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.CancelBlackContact(ctx.Request.Context(), req.UserId, req.ContactId)
	JsonBack(ctx, msg, ret, nil)
}

func DeleteContact(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId    string `json:"user_id"`
		ContactId string `json:"contact_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.DeleteContact(ctx.Request.Context(), req.UserId, req.ContactId)
	JsonBack(ctx, msg, ret, nil)
}

func RefuseContactApply(ctx *lark_http.Context, next func()) {
	var req struct {
		ApplyId string `json:"apply_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.RefuseContactApply(ctx.Request.Context(), req.ApplyId)
	JsonBack(ctx, msg, ret, nil)
}

func BlackApply(ctx *lark_http.Context, next func()) {
	var req struct {
		ApplyId string `json:"apply_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.BlackApply(ctx.Request.Context(), req.ApplyId)
	JsonBack(ctx, msg, ret, nil)
}

func GetAddGroupList(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId string `json:"user_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetAddGroupList(ctx.Request.Context(), req.UserId)
	JsonBack(ctx, msg, ret, data)
}
