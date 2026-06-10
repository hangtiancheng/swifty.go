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

type ContactInfoResponse struct {
	ContactId        string   `json:"contact_id"`
	ContactName      string   `json:"contact_name"`
	ContactAvatar    string   `json:"contact_avatar"`
	ContactPhone     string   `json:"contact_phone"`
	ContactEmail     string   `json:"contact_email"`
	ContactGender    int8     `json:"contact_gender"`
	ContactSignature string   `json:"contact_signature"`
	ContactBirthday  string   `json:"contact_birthday"`
	ContactNotice    string   `json:"contact_notice"`
	ContactMembers   []string `json:"contact_members"`
	ContactMemberCnt int      `json:"contact_member_cnt"`
	ContactOwnerId   string   `json:"contact_owner_id"`
	ContactAddMode   int8     `json:"contact_add_mode"`
}

type ContactApplyResponse struct {
	ApplyId     string `json:"apply_id"`
	UserId      string `json:"user_id"`
	ContactId   string `json:"contact_id"`
	ContactName string `json:"contact_name"`
	ContactType int8   `json:"contact_type"`
	Status      int8   `json:"status"`
	Message     string `json:"message"`
}

type ContactListItem struct {
	UserId   string `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

func GetUserList(ctx context.Context, ownerId string) (string, []ContactListItem, int) {
	var contacts []model.UserContact
	err := dao.ActiveQuery(&contacts).
		Where("user_id", ownerId).
		Where("contact_type", 0).
		Find(ctx, &contacts)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []ContactListItem
	for _, c := range contacts {
		if c.Status == constant.ContactNormal {
			var user model.UserInfo
			if err := dao.ActiveQuery(&user).Where("uuid", c.ContactId).First(ctx, &user); err == nil {
				list = append(list, ContactListItem{
					UserId: user.Uuid, Nickname: user.Nickname, Avatar: user.Avatar,
				})
			}
		}
	}
	return "success", list, 0
}

func LoadMyJoinedGroup(ctx context.Context, ownerId string) (string, []GroupListItem, int) {
	var contacts []model.UserContact
	err := dao.ActiveQuery(&contacts).
		Where("user_id", ownerId).
		Where("contact_type", 1).
		Find(ctx, &contacts)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []GroupListItem
	for _, c := range contacts {
		if c.Status == constant.ContactNormal {
			var group model.GroupInfo
			if err := dao.ActiveQuery(&group).Where("uuid", c.ContactId).First(ctx, &group); err == nil {
				list = append(list, GroupListItem{
					Uuid: group.Uuid, Name: group.Name, MemberCnt: group.MemberCnt,
					OwnerId: group.OwnerId, Avatar: group.Avatar,
				})
			}
		}
	}
	return "success", list, 0
}

func GetContactInfo(ctx context.Context, userId, contactId string) (string, *ContactInfoResponse, int) {
	if len(contactId) > 0 && contactId[0] == 'G' {
		var group model.GroupInfo
		if err := dao.ActiveQuery(&group).Where("uuid", contactId).First(ctx, &group); err != nil {
			log.Println(err)
			return constant.SystemError, nil, -1
		}
		return "success", &ContactInfoResponse{
			ContactId:        group.Uuid,
			ContactName:      group.Name,
			ContactAvatar:    group.Avatar,
			ContactNotice:    group.Notice,
			ContactMembers:   group.Members,
			ContactMemberCnt: group.MemberCnt,
			ContactOwnerId:   group.OwnerId,
			ContactAddMode:   group.AddMode,
		}, 0
	}
	var user model.UserInfo
	if err := dao.ActiveQuery(&user).Where("uuid", contactId).First(ctx, &user); err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	return "success", &ContactInfoResponse{
		ContactId:        user.Uuid,
		ContactName:      user.Nickname,
		ContactAvatar:    user.Avatar,
		ContactPhone:     user.Telephone,
		ContactEmail:     user.Email,
		ContactGender:    user.Gender,
		ContactSignature: user.Signature,
		ContactBirthday:  user.Birthday,
	}, 0
}

func ApplyContact(ctx context.Context, userId, contactId string, contactType int8, message string) (string, int) {
	apply := model.ContactApply{
		Uuid:        "A" + util.GetNowAndLenRandomString(11),
		UserId:      userId,
		ContactId:   contactId,
		ContactType: contactType,
		Status:      constant.ApplyStatusApplying,
		Message:     message,
		LastApplyAt: time.Now(),
	}
	if _, err := dao.Engine.Model(&apply).Insert(ctx, &apply); err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "application submitted", 0
}

func GetNewContactList(ctx context.Context, userId string) (string, []ContactApplyResponse, int) {
	var applies []model.ContactApply
	err := dao.ActiveQuery(&applies).Where("contact_id", userId).OrderBy("last_apply_at", "desc").Find(ctx, &applies)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []ContactApplyResponse
	for _, a := range applies {
		contactName := a.UserId
		var user model.UserInfo
		if err := dao.ActiveQuery(&user).Where("uuid", a.UserId).First(ctx, &user); err == nil {
			contactName = user.Nickname
		}
		list = append(list, ContactApplyResponse{
			ApplyId: a.Uuid, UserId: a.UserId, ContactId: a.ContactId,
			ContactName: contactName, ContactType: a.ContactType,
			Status: a.Status, Message: a.Message,
		})
	}
	return "success", list, 0
}

func PassContactApply(ctx context.Context, applyId string) (string, int) {
	var apply model.ContactApply
	err := dao.ActiveQuery(&apply).Where("uuid", applyId).First(ctx, &apply)
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	_, _ = dao.Engine.Model(&apply).Where("uuid", applyId).Update(ctx, bson.M{"status": constant.ApplyStatusPass})

	now := time.Now()
	c1 := model.UserContact{UserId: apply.UserId, ContactId: apply.ContactId, ContactType: apply.ContactType, Status: 0, CreatedAt: now, UpdateAt: now}
	c2 := model.UserContact{UserId: apply.ContactId, ContactId: apply.UserId, ContactType: apply.ContactType, Status: 0, CreatedAt: now, UpdateAt: now}
	_, _ = dao.Engine.Model(&c1).Insert(ctx, &c1)
	_, _ = dao.Engine.Model(&c2).Insert(ctx, &c2)
	return "application approved", 0
}

func BlackContact(ctx context.Context, userId, contactId string) (string, int) {
	now := time.Now()
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", userId).Where("contact_id", contactId).
		Update(ctx, bson.M{"status": constant.ContactBlack, "update_at": now})
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", contactId).Where("contact_id", userId).
		Update(ctx, bson.M{"status": constant.ContactBeBlack, "update_at": now})
	return "contact blocked", 0
}

func CancelBlackContact(ctx context.Context, userId, contactId string) (string, int) {
	now := time.Now()
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", userId).Where("contact_id", contactId).
		Update(ctx, bson.M{"status": constant.ContactNormal, "update_at": now})
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", contactId).Where("contact_id", userId).
		Update(ctx, bson.M{"status": constant.ContactNormal, "update_at": now})
	return "contact unblocked", 0
}

func DeleteContact(ctx context.Context, userId, contactId string) (string, int) {
	now := time.Now()
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", userId).Where("contact_id", contactId).
		Update(ctx, bson.M{"deleted_at": now})
	_, _ = dao.Engine.Model(&model.UserContact{}).
		Where("user_id", contactId).Where("contact_id", userId).
		Update(ctx, bson.M{"deleted_at": now})
	return "deleted", 0
}

func RefuseContactApply(ctx context.Context, applyId string) (string, int) {
	_, err := dao.Engine.Model(&model.ContactApply{}).Where("uuid", applyId).Update(ctx, bson.M{"status": constant.ApplyStatusRefuse})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "application refused", 0
}

func BlackApply(ctx context.Context, applyId string) (string, int) {
	_, err := dao.Engine.Model(&model.ContactApply{}).Where("uuid", applyId).Update(ctx, bson.M{"status": constant.ApplyStatusBlack})
	if err != nil {
		log.Println(err)
		return constant.SystemError, -1
	}
	return "application blocked", 0
}

func GetAddGroupList(ctx context.Context, userId string) (string, []ContactApplyResponse, int) {
	var applies []model.ContactApply
	err := dao.ActiveQuery(&applies).
		Where("contact_id", userId).
		Where("contact_type", int8(1)).
		OrderBy("last_apply_at", "desc").
		Find(ctx, &applies)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []ContactApplyResponse
	for _, a := range applies {
		contactName := a.UserId
		var user model.UserInfo
		if err := dao.ActiveQuery(&user).Where("uuid", a.UserId).First(ctx, &user); err == nil {
			contactName = user.Nickname
		}
		list = append(list, ContactApplyResponse{
			ApplyId: a.Uuid, UserId: a.UserId, ContactId: a.ContactId,
			ContactName: contactName, ContactType: a.ContactType,
			Status: a.Status, Message: a.Message,
		})
	}
	return "success", list, 0
}
