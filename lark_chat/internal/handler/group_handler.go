package handler

import (
	"go.mongodb.org/mongo-driver/bson"

	"github.com/hangtiancheng/lark-go/lark_http"
	"lark_chat/internal/service"
)

func CreateGroup(ctx *lark_http.Context, next func()) {
	var req struct {
		Name    string `json:"name"`
		OwnerId string `json:"owner_id"`
		Avatar  string `json:"avatar"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.CreateGroup(ctx.Request.Context(), req.Name, req.OwnerId, req.Avatar)
	JsonBack(ctx, msg, ret, data)
}

func LoadMyGroup(ctx *lark_http.Context, next func()) {
	var req struct {
		OwnerId string `json:"owner_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.LoadMyGroup(ctx.Request.Context(), req.OwnerId)
	JsonBack(ctx, msg, ret, data)
}

func GetGroupInfo(ctx *lark_http.Context, next func()) {
	var req struct {
		Uuid string `json:"uuid"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetGroupInfo(ctx.Request.Context(), req.Uuid)
	JsonBack(ctx, msg, ret, data)
}

func CheckGroupAddMode(ctx *lark_http.Context, next func()) {
	var req struct {
		GroupId string `json:"group_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, addMode, ret := service.CheckGroupAddMode(ctx.Request.Context(), req.GroupId)
	JsonBack(ctx, msg, ret, addMode)
}

func EnterGroupDirectly(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId  string `json:"user_id"`
		GroupId string `json:"group_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.EnterGroupDirectly(ctx.Request.Context(), req.UserId, req.GroupId)
	JsonBack(ctx, msg, ret, nil)
}

func LeaveGroup(ctx *lark_http.Context, next func()) {
	var req struct {
		UserId  string `json:"user_id"`
		GroupId string `json:"group_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.LeaveGroup(ctx.Request.Context(), req.UserId, req.GroupId)
	JsonBack(ctx, msg, ret, nil)
}

func DismissGroup(ctx *lark_http.Context, next func()) {
	var req struct {
		GroupId string `json:"group_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.DismissGroup(ctx.Request.Context(), req.GroupId)
	JsonBack(ctx, msg, ret, nil)
}

func UpdateGroupInfo(ctx *lark_http.Context, next func()) {
	var req struct {
		Uuid   string `json:"uuid"`
		Name   string `json:"name"`
		Notice string `json:"notice"`
		Avatar string `json:"avatar"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	fields := bson.M{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.Notice != "" {
		fields["notice"] = req.Notice
	}
	if req.Avatar != "" {
		fields["avatar"] = req.Avatar
	}
	msg, ret := service.UpdateGroupInfo(ctx.Request.Context(), req.Uuid, fields)
	JsonBack(ctx, msg, ret, nil)
}

func GetGroupMemberList(ctx *lark_http.Context, next func()) {
	var req struct {
		GroupId string `json:"group_id"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, data, ret := service.GetGroupMemberList(ctx.Request.Context(), req.GroupId)
	JsonBack(ctx, msg, ret, data)
}

func RemoveGroupMembers(ctx *lark_http.Context, next func()) {
	var req struct {
		GroupId   string   `json:"group_id"`
		MemberIds []string `json:"member_ids"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.RemoveGroupMembers(ctx.Request.Context(), req.GroupId, req.MemberIds)
	JsonBack(ctx, msg, ret, nil)
}

func GetGroupInfoList(ctx *lark_http.Context, next func()) {
	msg, data, ret := service.GetGroupInfoList(ctx.Request.Context())
	JsonBack(ctx, msg, ret, data)
}

func DeleteGroups(ctx *lark_http.Context, next func()) {
	var req struct {
		UuidList []string `json:"uuid_list"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.DeleteGroups(ctx.Request.Context(), req.UuidList)
	JsonBack(ctx, msg, ret, nil)
}

func SetGroupsStatus(ctx *lark_http.Context, next func()) {
	var req struct {
		UuidList []string `json:"uuid_list"`
		Status   int8     `json:"status"`
	}
	if err := ctx.BindJSON(&req); err != nil {
		JsonBack(ctx, "invalid request body", -1, nil)
		return
	}
	msg, ret := service.SetGroupsStatus(ctx.Request.Context(), req.UuidList, req.Status)
	JsonBack(ctx, msg, ret, nil)
}
