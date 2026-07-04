package service

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/constant"
	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/dao"
	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/model"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/util"
)

type GroupInfoResponse struct {
	Uuid      string   `json:"uuid"`
	Name      string   `json:"name"`
	Notice    string   `json:"notice"`
	Members   []string `json:"members"`
	MemberCnt int      `json:"member_cnt"`
	OwnerId   string   `json:"owner_id"`
	AddMode   int8     `json:"add_mode"`
	Avatar    string   `json:"avatar"`
	Status    int8     `json:"status"`
}

type GroupListItem struct {
	GroupId   string `json:"group_id"`
	Name      string `json:"name"`
	MemberCnt int    `json:"member_cnt"`
	OwnerId   string `json:"owner_id"`
	Avatar    string `json:"avatar"`
}

func CreateGroup(ctx context.Context, name, ownerId, avatar string) (string, *GroupInfoResponse, int) {
	group := model.GroupInfo{
		Uuid:      "G" + util.GetNowAndLenRandomString(11),
		Name:      name,
		Members:   []string{ownerId},
		MemberCnt: 1,
		OwnerId:   ownerId,
		Avatar:    avatar,
		Status:    constant.GroupStatusNormal,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if group.Avatar == "" {
		group.Avatar = "https://vitejs.dev/logo.svg"
	}
	if _, err := dao.Engine.Model(&group).Insert(ctx, &group); err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}

	contact := model.UserContact{
		UserId:      ownerId,
		ContactId:   group.Uuid,
		ContactType: 1,
		Status:      0,
		CreatedAt:   time.Now(),
		UpdateAt:    time.Now(),
	}
	_, _ = dao.Engine.Model(&contact).Insert(ctx, &contact)

	return "group created", toGroupInfoResponse(&group), 0
}

func GetGroupInfo(ctx context.Context, uuid string) (string, *GroupInfoResponse, int) {
	var group model.GroupInfo
	err := dao.ActiveQuery(&group).Where("uuid", uuid).First(ctx, &group)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	return "group info retrieved", toGroupInfoResponse(&group), 0
}

func LoadMyGroup(ctx context.Context, ownerId string) (string, []GroupListItem, int) {
	var groups []model.GroupInfo
	err := dao.ActiveQuery(&groups).Where("owner_id", ownerId).Find(ctx, &groups)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []GroupListItem
	for _, g := range groups {
		list = append(list, GroupListItem{
			GroupId: g.Uuid, Name: g.Name, MemberCnt: g.MemberCnt, OwnerId: g.OwnerId, Avatar: g.Avatar,
		})
	}
	return "success", list, 0
}

func CheckGroupAddMode(ctx context.Context, groupId string) (string, int8, int) {
	var group model.GroupInfo
	err := dao.ActiveQuery(&group).Where("uuid", groupId).First(ctx, &group)
	if err != nil {
		log.Println(err)
		return constant.SystemError, 0, -1
	}
	return "success", group.AddMode, 0
}

func EnterGroupDirectly(ctx context.Context, userId, groupId string) (string, int) {
	var group model.GroupInfo
	err := dao.ActiveQuery(&group).Where("uuid", groupId).First(ctx, &group)
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	group.Members = append(group.Members, userId)
	group.MemberCnt++
	_, err = dao.Engine.Model(&group).Where("uuid", groupId).Update(ctx, bson.M{
		"members": group.Members, "member_cnt": group.MemberCnt, "updated_at": time.Now(),
	})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}

	contact := model.UserContact{
		UserId: userId, ContactId: groupId, ContactType: 1, Status: 0,
		CreatedAt: time.Now(), UpdateAt: time.Now(),
	}
	_, _ = dao.Engine.Model(&contact).Insert(ctx, &contact)
	return "joined group", 0
}

func LeaveGroup(ctx context.Context, userId, groupId string) (string, int) {
	var group model.GroupInfo
	err := dao.ActiveQuery(&group).Where("uuid", groupId).First(ctx, &group)
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	newMembers := make([]string, 0, len(group.Members))
	for _, m := range group.Members {
		if m != userId {
			newMembers = append(newMembers, m)
		}
	}
	group.Members = newMembers
	group.MemberCnt = len(newMembers)
	_, _ = dao.Engine.Model(&group).Where("uuid", groupId).Update(ctx, bson.M{
		"members": group.Members, "member_cnt": group.MemberCnt, "updated_at": time.Now(),
	})

	now := time.Now()
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", userId).Where("contact_id", groupId).
		Update(ctx, bson.M{"deleted_at": now})

	return "left group", 0
}

func DismissGroup(ctx context.Context, groupId string) (string, int) {
	_, err := dao.Engine.Model(&model.GroupInfo{}).Where("uuid", groupId).Update(ctx, bson.M{
		"status": constant.GroupStatusDismiss, "deleted_at": time.Now(),
	})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "group dismissed", 0
}

func UpdateGroupInfo(ctx context.Context, uuid string, fields bson.M) (string, int) {
	fields["updated_at"] = time.Now()
	_, err := dao.Engine.Model(&model.GroupInfo{}).Where("uuid", uuid).Update(ctx, fields)
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "group info updated", 0
}

func GetGroupMemberList(ctx context.Context, groupId string) (string, []UserInfoResponse, int) {
	var group model.GroupInfo
	err := dao.ActiveQuery(&group).Where("uuid", groupId).First(ctx, &group)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var users []model.UserInfo
	err = dao.ActiveQuery(&users).WhereIn("uuid", group.Members).Find(ctx, &users)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []UserInfoResponse
	for _, u := range users {
		list = append(list, *toUserInfoResponse(&u))
	}
	return "success", list, 0
}

func RemoveGroupMembers(ctx context.Context, groupId string, memberIds []string) (string, int) {
	var group model.GroupInfo
	err := dao.ActiveQuery(&group).Where("uuid", groupId).First(ctx, &group)
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	removeSet := make(map[string]bool)
	for _, id := range memberIds {
		removeSet[id] = true
	}
	newMembers := make([]string, 0)
	for _, m := range group.Members {
		if !removeSet[m] {
			newMembers = append(newMembers, m)
		}
	}
	_, _ = dao.Engine.Model(&group).Where("uuid", groupId).Update(ctx, bson.M{
		"members": newMembers, "member_cnt": len(newMembers), "updated_at": time.Now(),
	})

	now := time.Now()
	for _, id := range memberIds {
		_, _ = dao.Engine.Model(&model.UserContact{}).
			Where("user_id", id).Where("contact_id", groupId).
			Update(ctx, bson.M{"deleted_at": now})
	}
	return "members removed", 0
}

func GetGroupInfoList(ctx context.Context) (string, []GroupListItem, int) {
	var groups []model.GroupInfo
	err := dao.Engine.Model(&groups).Find(ctx, &groups)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []GroupListItem
	for _, g := range groups {
		list = append(list, GroupListItem{
			GroupId: g.Uuid, Name: g.Name, MemberCnt: g.MemberCnt, OwnerId: g.OwnerId, Avatar: g.Avatar,
		})
	}
	return "success", list, 0
}

func DeleteGroups(ctx context.Context, uuidList []string) (string, int) {
	now := time.Now()
	_, err := dao.Engine.Model(&model.GroupInfo{}).WhereIn("uuid", uuidList).Update(ctx, bson.M{"deleted_at": now})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "groups deleted", 0
}

func SetGroupsStatus(ctx context.Context, uuidList []string, status int8) (string, int) {
	_, err := dao.Engine.Model(&model.GroupInfo{}).WhereIn("uuid", uuidList).Update(ctx, bson.M{"status": status, "updated_at": time.Now()})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "group status updated", 0
}

func toGroupInfoResponse(g *model.GroupInfo) *GroupInfoResponse {
	return &GroupInfoResponse{
		Uuid: g.Uuid, Name: g.Name, Notice: g.Notice, Members: g.Members,
		MemberCnt: g.MemberCnt, OwnerId: g.OwnerId, AddMode: g.AddMode,
		Avatar: g.Avatar, Status: g.Status,
	}
}
