import AppService from "./index";
import { BASE_URL } from "@/config";

AppService.add([
  // Auth
  { name: "login", url: BASE_URL + "/login" },
  { name: "register", url: BASE_URL + "/register" },

  // User
  { name: "getUserInfo", url: BASE_URL + "/user/get-user-info", cache: 30_000 },
  { name: "getUserInfoList", url: BASE_URL + "/user/get-user-info-list" },
  { name: "updateUserInfo", url: BASE_URL + "/user/update-user-info" },
  { name: "wsLogout", url: BASE_URL + "/user/ws-logout" },
  { name: "ableUsers", url: BASE_URL + "/user/able-users" },
  { name: "disableUsers", url: BASE_URL + "/user/disable-users" },
  { name: "deleteUsers", url: BASE_URL + "/user/delete-users" },
  { name: "setAdmin", url: BASE_URL + "/user/set-admin" },

  // Contact
  {
    name: "getContactInfo",
    url: BASE_URL + "/contact/get-contact-info",
    cache: 15_000,
  },
  { name: "getUserList", url: BASE_URL + "/contact/get-user-list" },
  { name: "applyContact", url: BASE_URL + "/contact/apply-contact" },
  { name: "passContactApply", url: BASE_URL + "/contact/pass-contact-apply" },
  {
    name: "getNewContactList",
    url: BASE_URL + "/contact/get-new-contact-list",
  },
  {
    name: "refuseContactApply",
    url: BASE_URL + "/contact/refuse-contact-apply",
  },
  { name: "blackApply", url: BASE_URL + "/contact/black-apply" },
  { name: "getAddGroupList", url: BASE_URL + "/contact/get-add-group-list" },
  {
    name: "deleteContact",
    url: BASE_URL + "/contact/delete-contact",
    cleanKeys: "getUserList",
  },
  { name: "blackContact", url: BASE_URL + "/contact/black-contact" },
  {
    name: "cancelBlackContact",
    url: BASE_URL + "/contact/cancel-black-contact",
  },
  {
    name: "loadMyJoinedGroup",
    url: BASE_URL + "/contact/load-my-joined-group",
  },

  // Session
  { name: "openSession", url: BASE_URL + "/session/open-session" },
  {
    name: "getUserSessionList",
    url: BASE_URL + "/session/get-user-session-list",
  },
  {
    name: "getGroupSessionList",
    url: BASE_URL + "/session/get-group-session-list",
  },
  {
    name: "deleteSession",
    url: BASE_URL + "/session/delete-session",
    cleanKeys: "getUserSessionList,getGroupSessionList",
  },
  {
    name: "checkOpenSessionAllowed",
    url: BASE_URL + "/session/check-open-session-allowed",
  },

  // Message
  { name: "getMessageList", url: BASE_URL + "/message/get-message-list" },
  {
    name: "getGroupMessageList",
    url: BASE_URL + "/message/get-group-message-list",
  },
  { name: "uploadFile", url: BASE_URL + "/message/upload-file" },
  { name: "uploadAvatar", url: BASE_URL + "/message/upload-avatar" },

  // Group
  { name: "createGroup", url: BASE_URL + "/group/create-group" },
  { name: "loadMyGroup", url: BASE_URL + "/group/load-my-group" },
  { name: "getGroupInfo", url: BASE_URL + "/group/get-group-info" },
  {
    name: "getGroupMemberList",
    url: BASE_URL + "/group/get-group-member-list",
  },
  { name: "updateGroupInfo", url: BASE_URL + "/group/update-group-info" },
  { name: "removeGroupMembers", url: BASE_URL + "/group/remove-group-members" },
  { name: "leaveGroup", url: BASE_URL + "/group/leave-group" },
  { name: "dismissGroup", url: BASE_URL + "/group/dismiss-group" },
  { name: "checkGroupAddMode", url: BASE_URL + "/group/check-group-add-mode" },
  { name: "enterGroupDirectly", url: BASE_URL + "/group/enter-group-directly" },

  // Group (admin)
  { name: "getGroupInfoList", url: BASE_URL + "/group/get-group-info-list" },
  { name: "deleteGroups", url: BASE_URL + "/group/delete-groups" },
  { name: "setGroupsStatus", url: BASE_URL + "/group/set-groups-status" },

  // Chatroom
  { name: "getOnlineUsers", url: BASE_URL + "/chatroom/get-online-users" },
]);
