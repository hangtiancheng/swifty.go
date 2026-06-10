package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"lark_chat/internal/constant"
	"lark_chat/internal/dao"
	"lark_chat/internal/model"

	"lark_chat/internal/util"
)

type UserInfoResponse struct {
	Uuid      string `json:"uuid"`
	Telephone string `json:"telephone"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Avatar    string `json:"avatar"`
	Gender    int8   `json:"gender"`
	Birthday  string `json:"birthday"`
	Signature string `json:"signature"`
	IsAdmin   int8   `json:"is_admin"`
	Status    int8   `json:"status"`
	CreatedAt string `json:"created_at"`
}

type UserListItem struct {
	Uuid      string `json:"uuid"`
	Telephone string `json:"telephone"`
	Nickname  string `json:"nickname"`
	Status    int8   `json:"status"`
	IsAdmin   int8   `json:"is_admin"`
	IsDeleted bool   `json:"is_deleted"`
}

func Login(ctx context.Context, telephone string, password string) (string, *UserInfoResponse, int) {
	var user model.UserInfo
	err := dao.ActiveQuery(&user).Where("telephone", telephone).First(ctx, &user)
	if err != nil {
		log.Println(err)
		return "user not found, please register", nil, -2
	}
	if user.Password != password {
		return "incorrect password", nil, -2
	}
	return "login successful", toUserInfoResponse(&user), 0
}

func Register(ctx context.Context, telephone, password, nickname string) (string, *UserInfoResponse, int) {
	var existing model.UserInfo
	err := dao.ActiveQuery(&existing).Where("telephone", telephone).First(ctx, &existing)
	if err == nil {
		return "phone number already registered", nil, -2
	}

	user := model.UserInfo{
		Uuid:      "U" + util.GetNowAndLenRandomString(11),
		Telephone: telephone,
		Password:  password,
		Nickname:  nickname,
		Avatar:    "https://vitejs.dev/logo.svg",
		CreatedAt: time.Now(),
		IsAdmin:   0,
		Status:    constant.UserStatusNormal,
	}

	if _, err := dao.Engine.Model(&user).Insert(ctx, &user); err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	return "registration successful", toUserInfoResponse(&user), 0
}

func UpdateUserInfo(ctx context.Context, uuid string, fields bson.M) (string, int) {
	if len(fields) == 0 {
		return "user info updated", 0
	}
	_, err := dao.Engine.Model(&model.UserInfo{}).Where("uuid", uuid).Update(ctx, fields)
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	_ = dao.UserInfoCache.Delete(ctx, uuid)
	return "user info updated", 0
}

func GetUserInfo(ctx context.Context, uuid string) (string, *UserInfoResponse, int) {
	var user model.UserInfo
	err := dao.ActiveQuery(&user).Where("uuid", uuid).First(ctx, &user)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	return "user info retrieved", toUserInfoResponse(&user), 0
}

func GetUserInfoList(ctx context.Context, ownerId string) (string, []UserListItem, int) {
	var users []model.UserInfo
	err := dao.Engine.Model(&users).Where("uuid", "!=", ownerId).Find(ctx, &users)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []UserListItem
	for _, u := range users {
		list = append(list, UserListItem{
			Uuid:      u.Uuid,
			Telephone: u.Telephone,
			Nickname:  u.Nickname,
			Status:    u.Status,
			IsAdmin:   u.IsAdmin,
			IsDeleted: u.DeletedAt != nil,
		})
	}
	return "user list retrieved", list, 0
}

func AbleUsers(ctx context.Context, uuidList []string) (string, int) {
	_, err := dao.Engine.Model(&model.UserInfo{}).WhereIn("uuid", uuidList).Update(ctx, bson.M{"status": constant.UserStatusNormal})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "users enabled", 0
}

func DisableUsers(ctx context.Context, uuidList []string) (string, int) {
	_, err := dao.Engine.Model(&model.UserInfo{}).WhereIn("uuid", uuidList).Update(ctx, bson.M{"status": constant.UserStatusDisable})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	now := time.Now()
	for _, uuid := range uuidList {
		_, _ = dao.Engine.Model(&model.Session{}).
			Where("send_id", uuid).
			WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
		_, _ = dao.Engine.Model(&model.Session{}).
			Where("receive_id", uuid).
			WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
	}
	return "users disabled", 0
}

func DeleteUsers(ctx context.Context, uuidList []string) (string, int) {
	now := time.Now()
	_, err := dao.Engine.Model(&model.UserInfo{}).WhereIn("uuid", uuidList).Update(ctx, bson.M{"deleted_at": now})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	for _, uuid := range uuidList {
		_, _ = dao.Engine.Model(&model.Session{}).
			Where("send_id", uuid).WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
		_, _ = dao.Engine.Model(&model.Session{}).
			Where("receive_id", uuid).WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
		_, _ = dao.Engine.Model(&model.UserContact{}).
			Where("user_id", uuid).WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
		_, _ = dao.Engine.Model(&model.UserContact{}).
			Where("contact_id", uuid).WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
		_, _ = dao.Engine.Model(&model.ContactApply{}).
			Where("user_id", uuid).WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
		_, _ = dao.Engine.Model(&model.ContactApply{}).
			Where("contact_id", uuid).WhereNull("deleted_at").
			Update(ctx, bson.M{"deleted_at": now})
	}
	return "users deleted", 0
}

func SetAdmin(ctx context.Context, uuidList []string, isAdmin int8) (string, int) {
	_, err := dao.Engine.Model(&model.UserInfo{}).WhereIn("uuid", uuidList).Update(ctx, bson.M{"is_admin": isAdmin})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "admin status updated", 0
}

func toUserInfoResponse(user *model.UserInfo) *UserInfoResponse {
	year, month, day := user.CreatedAt.Date()
	return &UserInfoResponse{
		Uuid:      user.Uuid,
		Telephone: user.Telephone,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Gender:    user.Gender,
		Birthday:  user.Birthday,
		Signature: user.Signature,
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
		CreatedAt: fmt.Sprintf("%d.%d.%d", year, month, day),
	}
}
