package service

import (
	"context"
	"log"

	"lark_chat/internal/constant"
	"lark_chat/internal/dao"
	"lark_chat/internal/model"
)

type MessageListItem struct {
	SendId     string `json:"send_id"`
	SendName   string `json:"send_name"`
	SendAvatar string `json:"send_avatar"`
	ReceiveId  string `json:"receive_id"`
	Type       int8   `json:"type"`
	Content    string `json:"content"`
	Url        string `json:"url"`
	FileSize   string `json:"file_size"`
	FileName   string `json:"file_name"`
	FileType   string `json:"file_type"`
	CreatedAt  string `json:"created_at"`
}

func GetMessageList(ctx context.Context, sendId, receiveId string) (string, []MessageListItem, int) {
	var messages []model.Message
	err := dao.Engine.Model(&messages).
		Where("send_id", sendId).
		Where("receive_id", receiveId).
		OrderBy("created_at", "asc").
		Find(ctx, &messages)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}

	var reverseMessages []model.Message
	err = dao.Engine.Model(&reverseMessages).
		Where("send_id", receiveId).
		Where("receive_id", sendId).
		OrderBy("created_at", "asc").
		Find(ctx, &reverseMessages)
	if err == nil {
		messages = append(messages, reverseMessages...)
	}

	var list []MessageListItem
	for _, m := range messages {
		list = append(list, MessageListItem{
			SendId: m.SendId, SendName: m.SendName, SendAvatar: m.SendAvatar,
			ReceiveId: m.ReceiveId, Type: m.Type, Content: m.Content,
			Url: m.Url, FileSize: m.FileSize, FileName: m.FileName,
			FileType: m.FileType, CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return "success", list, 0
}

func GetGroupMessageList(ctx context.Context, groupId string) (string, []MessageListItem, int) {
	var messages []model.Message
	err := dao.Engine.Model(&messages).
		Where("receive_id", groupId).
		OrderBy("created_at", "asc").
		Find(ctx, &messages)
	if err != nil {
		log.Println(err)
		return constant.SystemError, nil, -1
	}
	var list []MessageListItem
	for _, m := range messages {
		list = append(list, MessageListItem{
			SendId: m.SendId, SendName: m.SendName, SendAvatar: m.SendAvatar,
			ReceiveId: m.ReceiveId, Type: m.Type, Content: m.Content,
			Url: m.Url, FileSize: m.FileSize, FileName: m.FileName,
			FileType: m.FileType, CreatedAt: m.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return "success", list, 0
}
