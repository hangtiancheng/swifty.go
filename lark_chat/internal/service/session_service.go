package service

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"lark_chat/internal/constant"
	"lark_chat/internal/dao"
	"lark_chat/internal/model"
	"lark_chat/internal/util"
)

type UserSessionItem struct {
	SessionId string `json:"session_id"`
	Avatar    string `json:"avatar"`
	UserId    string `json:"user_id"`
	Username  string `json:"username"`
}

type GroupSessionItem struct {
	SessionId string `json:"session_id"`
	Avatar    string `json:"avatar"`
	GroupId   string `json:"group_id"`
	GroupName string `json:"group_name"`
}

func OpenSession(ctx context.Context, sendId, receiveId string) (string, string, int) {
	var session model.Session
	err := dao.ActiveQuery(&session).
		Where("send_id", sendId).
		Where("receive_id", receiveId).
		First(ctx, &session)
	if err == nil {
		return "session created", session.Uuid, 0
	}
	return createSession(ctx, sendId, receiveId)
}

func createSession(ctx context.Context, sendId, receiveId string) (string, string, int) {
	session := model.Session{
		Uuid:      "S" + util.GetNowAndLenRandomString(11),
		SendId:    sendId,
		ReceiveId: receiveId,
		CreatedAt: time.Now(),
	}

	if receiveId[0] == 'U' {
		var user model.UserInfo
		if err := dao.ActiveQuery(&user).Where("uuid", receiveId).First(ctx, &user); err != nil {
			log.Println(err)
			return constant.SystemError, "", -1
		}
		session.ReceiveName = user.Nickname
		session.Avatar = user.Avatar
	} else {
		var group model.GroupInfo
		if err := dao.ActiveQuery(&group).Where("uuid", receiveId).First(ctx, &group); err != nil {
			log.Println(err)
			return constant.SystemError, "", -1
		}
		session.ReceiveName = group.Name
		session.Avatar = group.Avatar
	}

	if _, err := dao.Engine.Model(&session).Insert(ctx, &session); err != nil {
		log.Println(err)
		return constant.SystemError, "", -1
	}
	_ = dao.SessionListCache.Delete(ctx, sendId)
	_ = dao.GrpSessionListCache.Delete(ctx, sendId)
	return "session created", session.Uuid, 0
}

func GetUserSessionList(ctx context.Context, ownerId string) (string, []UserSessionItem, int) {
	var sessions []model.Session
	err := dao.ActiveQuery(&sessions).
		Where("send_id", ownerId).
		OrderBy("created_at", "desc").
		Find(ctx, &sessions)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []UserSessionItem
	for _, s := range sessions {
		if len(s.ReceiveId) > 0 && s.ReceiveId[0] == 'U' {
			list = append(list, UserSessionItem{
				SessionId: s.Uuid, Avatar: s.Avatar, UserId: s.ReceiveId, Username: s.ReceiveName,
			})
		}
	}
	return "success", list, 0
}

func GetGroupSessionList(ctx context.Context, ownerId string) (string, []GroupSessionItem, int) {
	var sessions []model.Session
	err := dao.ActiveQuery(&sessions).
		Where("send_id", ownerId).
		OrderBy("created_at", "desc").
		Find(ctx, &sessions)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []GroupSessionItem
	for _, s := range sessions {
		if len(s.ReceiveId) > 0 && s.ReceiveId[0] == 'G' {
			list = append(list, GroupSessionItem{
				SessionId: s.Uuid, Avatar: s.Avatar, GroupId: s.ReceiveId, GroupName: s.ReceiveName,
			})
		}
	}
	return "success", list, 0
}

func DeleteSession(ctx context.Context, ownerId, sessionId string) (string, int) {
	now := time.Now()
	_, err := dao.Engine.Model(&model.Session{}).Where("uuid", sessionId).Update(ctx, bson.M{"deleted_at": now})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	_ = dao.SessionListCache.Delete(ctx, ownerId)
	_ = dao.GrpSessionListCache.Delete(ctx, ownerId)
	return "deleted", 0
}

func CheckOpenSessionAllowed(ctx context.Context, sendId, receiveId string) (string, bool, int) {
	var contact model.UserContact
	err := dao.ActiveQuery(&contact).Where("user_id", sendId).Where("contact_id", receiveId).First(ctx, &contact)
	if err != nil {
		log.Println(err)
		return constant.SystemError, false, -1
	}
	if contact.Status == constant.ContactBeBlack {
		return "blocked by the other user", false, -2
	}
	if contact.Status == constant.ContactBlack {
		return "unblock the user first", false, -2
	}
	if receiveId[0] == 'U' {
		var user model.UserInfo
		if err := dao.ActiveQuery(&user).Where("uuid", receiveId).First(ctx, &user); err != nil {
			return constant.SystemError, false, -1
		}
		if user.Status == constant.UserStatusDisable {
			return "target user is disabled", false, -2
		}
	} else {
		var group model.GroupInfo
		if err := dao.ActiveQuery(&group).Where("uuid", receiveId).First(ctx, &group); err != nil {
			return constant.SystemError, false, -1
		}
		if group.Status == constant.GroupStatusDisable {
			return "target group is disabled", false, -2
		}
	}
	return "allowed", true, 0
}
