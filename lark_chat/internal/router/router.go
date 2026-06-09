package router

import (
	"github.com/hangtiancheng/lark-go/lark_http"
	"lark_chat/internal/config"
	"lark_chat/internal/handler"
	"lark_chat/internal/middleware"
)

func Setup() *lark_http.Application {
	app := lark_http.Default()
	app.Use(middleware.CORS())

	conf := config.Get()
	app.Static("/static/avatars", conf.Static.AvatarPath)
	app.Static("/static/files", conf.Static.FilePath)

	app.Post("/login", handler.Login)
	app.Post("/register", handler.Register)

	user := app.Router("/user")
	user.Post("/update-user-info", handler.UpdateUserInfo)
	user.Post("/get-user-info-list", handler.GetUserInfoList)
	user.Post("/able-users", handler.AbleUsers)
	user.Post("/get-user-info", handler.GetUserInfo)
	user.Post("/disable-users", handler.DisableUsers)
	user.Post("/delete-users", handler.DeleteUsers)
	user.Post("/set-admin", handler.SetAdmin)
	user.Post("/ws-logout", handler.WsLogout)

	group := app.Router("/group")
	group.Post("/create-group", handler.CreateGroup)
	group.Post("/load-my-group", handler.LoadMyGroup)
	group.Post("/check-group-add-mode", handler.CheckGroupAddMode)
	group.Post("/enter-group-directly", handler.EnterGroupDirectly)
	group.Post("/leave-group", handler.LeaveGroup)
	group.Post("/dismiss-group", handler.DismissGroup)
	group.Post("/get-group-info", handler.GetGroupInfo)
	group.Post("/update-group-info", handler.UpdateGroupInfo)
	group.Post("/get-group-member-list", handler.GetGroupMemberList)
	group.Post("/remove-group-members", handler.RemoveGroupMembers)
	group.Post("/get-group-info-list", handler.GetGroupInfoList)
	group.Post("/delete-groups", handler.DeleteGroups)
	group.Post("/set-groups-status", handler.SetGroupsStatus)

	session := app.Router("/session")
	session.Post("/open-session", handler.OpenSession)
	session.Post("/get-user-session-list", handler.GetUserSessionList)
	session.Post("/get-group-session-list", handler.GetGroupSessionList)
	session.Post("/delete-session", handler.DeleteSession)
	session.Post("/check-open-session-allowed", handler.CheckOpenSessionAllowed)

	contact := app.Router("/contact")
	contact.Post("/get-user-list", handler.GetUserList)
	contact.Post("/load-my-joined-group", handler.LoadMyJoinedGroup)
	contact.Post("/get-contact-info", handler.GetContactInfo)
	contact.Post("/apply-contact", handler.ApplyContact)
	contact.Post("/get-new-contact-list", handler.GetNewContactList)
	contact.Post("/pass-contact-apply", handler.PassContactApply)
	contact.Post("/refuse-contact-apply", handler.RefuseContactApply)
	contact.Post("/black-contact", handler.BlackContact)
	contact.Post("/cancel-black-contact", handler.CancelBlackContact)
	contact.Post("/black-apply", handler.BlackApply)
	contact.Post("/get-add-group-list", handler.GetAddGroupList)
	contact.Post("/delete-contact", handler.DeleteContact)

	message := app.Router("/message")
	message.Post("/get-message-list", handler.GetMessageList)
	message.Post("/get-group-message-list", handler.GetGroupMessageList)
	message.Post("/upload-avatar", handler.UploadAvatar)
	message.Post("/upload-file", handler.UploadFile)

	chatroom := app.Router("/chatroom")
	chatroom.Post("/get-online-users", handler.GetOnlineUsers)

	app.Get("/wss", handler.WsLogin)

	return app
}
