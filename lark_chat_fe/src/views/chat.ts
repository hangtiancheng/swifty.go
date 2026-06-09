import { Router, Framework, Frame, defineView } from "@lark.js/mvc";
import template from "./chat.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import useChatStore from "@/store/chat";
import useWsStore from "@/store/ws";
import { resolveAvatar } from "@/utils/avatar";
import { showToast } from "@/utils/toast";
import { BASE_URL } from "@/config";
import type { ContactInfo, Message } from "@/types";

function getFileSize(size: number): string {
  if (size < 1024) return size + "B";
  if (size < 1024 * 1024) return (size / 1024).toFixed(2) + "KB";
  if (size < 1024 * 1024 * 1024) return (size / 1024 / 1024).toFixed(2) + "MB";
  return (size / 1024 / 1024 / 1024).toFixed(2) + "GB";
}

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,
  editGroupData: { name: "", notice: "", add_mode: -1, avatar: "" },
  groupAvatarFile: null as File | null,
  selectedMembers: [] as string[],

  init() {
    this.srv = new AppService();
    this.capture("srv", this.srv);

    this.observeLocation(["id"], true);

    this.updater.set({
      chatMessage: "",
      contactName: "",
      contactAvatar: "",
      contactId: "",
      isUserContact: false,
      isGroupContact: false,
      isGroupOwner: false,
      contactGenderText: "",
      contactPhone: "",
      contactEmail: "",
      contactBirthday: "",
      contactSignature: "",
      groupMemberCnt: 0,
      groupOwnerId: "",
      groupAddModeText: "",
      groupNotice: "",
      memberList: [],
      joinRequestList: [],
    });

    useChatStore().clearChat();

    const loc = Router.parse();
    const contactId = loc.get("id", "") as string;
    if (contactId) this.loadChat(contactId);

    useWsStore().setOnMessage((event: MessageEvent) => {
      this.handleWsMessage(event);
    });

    this.on("destroy", () => {
      useWsStore().setOnMessage(() => {});
    });
  },

  assign() {
    this.updater.snapshot();
    const chat = useChatStore();
    const auth = useAuthStore();
    const msgBubbleFrame = Framework.Frame?.get?.("chat-messages");
    if (msgBubbleFrame) {
      msgBubbleFrame.invoke("updater.set", [
        {
          messageList: chat.messageList,
          currentUserId: auth.userInfo.uuid,
          currentUserAvatar: auth.userInfo.avatar,
          currentUserName: auth.userInfo.nickname,
        },
      ]);
    }
    return this.updater.altered();
  },

  render() {
    this.updater.digest();
    this.scrollToBottom();
  },

  loadChat(contactId: string) {
    this.srv!.all(
      {
        name: "getContactInfo",
        data: { user_id: useAuthStore().userInfo.uuid, contact_id: contactId },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const info = payload.get("data") as ContactInfo;
        if (!info) return;
        info.contact_avatar = resolveAvatar(info.contact_avatar);
        useChatStore().setContact(info);

        const auth = useAuthStore();
        const isUser = info.contact_id.startsWith("U");
        this.updater
          .set({
            contactName: info.contact_name,
            contactAvatar: info.contact_avatar,
            contactId: info.contact_id,
            isUserContact: isUser,
            isGroupContact: !isUser,
            isGroupOwner: info.contact_owner_id === auth.userInfo.uuid,
            contactGenderText: isUser ? (info.contact_gender === 0 ? "Male" : "Female") : "",
            contactPhone: info.contact_phone || "",
            contactEmail: info.contact_email || "",
            contactBirthday: info.contact_birthday || "",
            contactSignature: info.contact_signature || "",
            groupMemberCnt: isUser ? 0 : info.contact_member_cnt,
            groupOwnerId: info.contact_owner_id || "",
            groupAddModeText: isUser
              ? ""
              : info.contact_add_mode === 0
                ? "Direct Join"
                : "Owner Approval",
            groupNotice: info.contact_notice || "",
          })
          .digest();

        this.loadSession(contactId);
      },
    );
  },

  loadSession(contactId: string) {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      { name: "openSession", data: { send_id: uid, receive_id: contactId } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const sessionId = payload.get("data") as string;
        useChatStore().setSessionId(sessionId);
        this.loadMessages(contactId);
      },
    );
  },

  loadMessages(contactId: string) {
    const uid = useAuthStore().userInfo.uuid;
    const isUser = contactId.startsWith("U");
    const endpointName = isUser ? "getMessageList" : "getGroupMessageList";
    const reqData = isUser ? { send_id: uid, receive_id: contactId } : { group_id: contactId };

    this.srv!.save(
      { name: endpointName, data: reqData },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const list = (payload.get("data") as Message[] | null) || [];
        list.forEach((m) => {
          m.send_avatar = resolveAvatar(m.send_avatar);
        });
        useChatStore().setMessageList(list);
        this.updater.set({ messageList: list }).digest();
      },
    );
  },

  handleWsMessage(event: MessageEvent) {
    const message = JSON.parse(event.data) as Message;
    const auth = useAuthStore();
    const chat = useChatStore();

    if (message.type === 3) {
      const avData = JSON.parse(message.av_data || "{}") as Record<string, unknown>;
      const vcFrames = Frame.getAll();
      for (const [, f] of vcFrames) {
        if (f.invoke) {
          f.invoke("handleSignal", [avData]);
          break;
        }
      }
      return;
    }

    const isRelevant =
      (message.receive_id.startsWith("G") && message.receive_id === chat.contactInfo?.contact_id) ||
      (message.receive_id.startsWith("U") && message.receive_id === auth.userInfo.uuid) ||
      message.send_id === auth.userInfo.uuid;

    if (isRelevant) {
      message.send_avatar = resolveAvatar(message.send_avatar);
      useChatStore().addMessage(message);
      this.updater.set({ messageList: useChatStore().messageList }).digest();
    }
  },

  "openVideoCall<click>"() {
    const vcFrames = Frame.getAll();
    for (const [, f] of vcFrames) {
      if (f.invoke) {
        f.invoke("show");
        break;
      }
    }
  },

  scrollToBottom() {
    const el = document.getElementById("chat-messages");
    if (el) el.scrollTop = el.scrollHeight;
  },

  "onMsgInput<input>"(e: Record<string, unknown>) {
    this.updater.set({
      chatMessage: (e.eventTarget as HTMLTextAreaElement).value,
    });
  },

  "sendMessage<click>"() {
    const content = this.updater.get("chatMessage") as string;
    if (!content) return;
    const auth = useAuthStore();
    const chat = useChatStore();
    const msg = {
      session_id: chat.sessionId,
      type: 0,
      content,
      url: "",
      send_id: auth.userInfo.uuid,
      send_name: auth.userInfo.nickname,
      send_avatar: auth.userInfo.avatar,
      receive_id: chat.contactInfo!.contact_id,
      file_size: getFileSize(0),
      file_name: "",
      file_type: "",
    };
    useWsStore().send(msg);
    this.updater.set({ chatMessage: "" }).digest();
  },

  "onFileSelect<change>"(e: Record<string, unknown>) {
    const input = e.eventTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    const formData = new FormData();
    formData.append("file", file);
    this.srv!.save({ name: "uploadFile", data: formData }, () => {
      showToast("File uploaded successfully", "success");
      const auth = useAuthStore();
      const chat = useChatStore();
      const msg = {
        session_id: chat.sessionId,
        type: 2,
        content: "",
        url: BASE_URL + "/static/files/" + file.name,
        send_id: auth.userInfo.uuid,
        send_name: auth.userInfo.nickname,
        send_avatar: auth.userInfo.avatar,
        receive_id: chat.contactInfo!.contact_id,
        file_size: getFileSize(file.size),
        file_name: file.name,
        file_type: file.type,
      };
      useWsStore().send(msg);
    });
    input.value = "";
  },

  "deleteSession<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    const chat = useChatStore();
    this.srv!.save(
      {
        name: "deleteSession",
        data: { owner_id: uid, session_id: chat.sessionId },
      },
      () => {
        Router.to("/chat/sessions");
      },
    );
  },

  "deleteContact<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    const chat = useChatStore();
    this.srv!.save(
      {
        name: "deleteContact",
        data: { user_id: uid, contact_id: chat.contactInfo!.contact_id },
      },
      () => {
        showToast("Contact removed", "success");
        Router.to("/chat/sessions");
      },
    );
  },

  "blackContact<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    const chat = useChatStore();
    this.srv!.save(
      {
        name: "blackContact",
        data: { user_id: uid, contact_id: chat.contactInfo!.contact_id },
      },
      () => {
        showToast("Contact blocked", "success");
        Router.to("/chat/sessions");
      },
    );
  },

  "dismissGroup<click>"() {
    const chat = useChatStore();
    this.srv!.save(
      {
        name: "dismissGroup",
        data: { group_id: chat.contactInfo!.contact_id },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Group disbanded", "success");
          Router.to("/chat/sessions");
        } else showToast(payload.get("message") as string, "error");
      },
    );
  },

  "leaveGroup<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    const chat = useChatStore();
    this.srv!.save(
      {
        name: "leaveGroup",
        data: { user_id: uid, group_id: chat.contactInfo!.contact_id },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Left group", "success");
          Router.to("/chat/sessions");
        } else showToast(payload.get("message") as string, "error");
      },
    );
  },

  // -- User Info Modal --
  "showUserInfoModal<click>"() {
    (document.getElementById("user-info-modal") as HTMLDialogElement)?.showModal();
  },
  "closeUserInfoModal<click>"() {
    (document.getElementById("user-info-modal") as HTMLDialogElement)?.close();
  },

  // -- Group Info Modal --
  "showGroupInfoModal<click>"() {
    (document.getElementById("group-info-modal") as HTMLDialogElement)?.showModal();
  },
  "closeGroupInfoModal<click>"() {
    (document.getElementById("group-info-modal") as HTMLDialogElement)?.close();
  },

  // -- Edit Group Modal --
  "showEditGroupModal<click>"() {
    this.editGroupData = { name: "", notice: "", add_mode: -1, avatar: "" };
    this.groupAvatarFile = null;
    (document.getElementById("edit-group-modal") as HTMLDialogElement)?.showModal();
  },
  "onEditGroupName<input>"(e: Record<string, unknown>) {
    this.editGroupData.name = (e.eventTarget as HTMLInputElement).value;
  },
  "onEditGroupNotice<input>"(e: Record<string, unknown>) {
    this.editGroupData.notice = (e.eventTarget as HTMLTextAreaElement).value;
  },
  "onEditGroupAddMode<change>"(e: Record<string, unknown>) {
    this.editGroupData.add_mode = Number((e.eventTarget as HTMLInputElement).value);
  },
  "onGroupAvatarSelect<change>"(e: Record<string, unknown>) {
    this.groupAvatarFile = (e.eventTarget as HTMLInputElement).files?.[0] ?? null;
  },
  "saveGroupInfo<click>"() {
    const d = this.editGroupData;
    if (!d.name && !d.notice && d.add_mode === -1 && !this.groupAvatarFile) {
      showToast("Please modify at least one field", "warning");
      return;
    }
    if (d.name && (d.name.length < 3 || d.name.length > 10)) {
      showToast("Group name must be 3-10 characters", "error");
      return;
    }
    if (this.groupAvatarFile) {
      const formData = new FormData();
      formData.append("file", this.groupAvatarFile);
      this.srv!.save({ name: "uploadAvatar", data: formData }, () => {});
      d.avatar = "/static/avatars/" + this.groupAvatarFile.name;
    }
    const chat = useChatStore();
    const data: Record<string, unknown> = {
      uuid: chat.contactInfo!.contact_id,
    };
    if (d.name) data.name = d.name;
    if (d.notice) data.notice = d.notice;
    if (d.add_mode !== -1) data.add_mode = d.add_mode;
    if (d.avatar) data.avatar = d.avatar;

    this.srv!.save(
      { name: "updateGroupInfo", data },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Group updated", "success");
          (document.getElementById("edit-group-modal") as HTMLDialogElement)?.close();
          this.loadChat(chat.contactInfo!.contact_id);
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },
  "closeEditGroupModal<click>"() {
    (document.getElementById("edit-group-modal") as HTMLDialogElement)?.close();
  },

  // -- Remove Members Modal --
  "showRemoveMembersModal<click>"() {
    const chat = useChatStore();
    this.selectedMembers = [];
    this.srv!.save(
      {
        name: "getGroupMemberList",
        data: { group_id: chat.contactInfo!.contact_id },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const list = (payload.get("data") as Array<Record<string, string>> | null) || [];
        list.forEach((m) => {
          m.avatar = resolveAvatar(m.avatar);
        });
        this.updater.set({ memberList: list }).digest();
        (document.getElementById("remove-members-modal") as HTMLDialogElement)?.showModal();
      },
    );
  },
  "toggleMember<change>"(e: Record<string, unknown>) {
    const id = (e.params as Record<string, string>).id;
    const idx = this.selectedMembers.indexOf(id);
    if (idx >= 0) this.selectedMembers.splice(idx, 1);
    else this.selectedMembers.push(id);
  },
  "removeSelectedMembers<click>"() {
    if (this.selectedMembers.length === 0) {
      showToast("Please select members to remove", "warning");
      return;
    }
    const chat = useChatStore();
    this.srv!.save(
      {
        name: "removeGroupMembers",
        data: {
          group_id: chat.contactInfo!.contact_id,
          member_ids: this.selectedMembers,
        },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Members removed", "success");
          const remaining = (
            this.updater.get("memberList") as Array<Record<string, string>>
          ).filter((m) => !this.selectedMembers.includes(m.user_id));
          this.selectedMembers = [];
          this.updater.set({ memberList: remaining }).digest();
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },
  "closeRemoveMembersModal<click>"() {
    (document.getElementById("remove-members-modal") as HTMLDialogElement)?.close();
  },

  // -- Join Requests Modal --
  "showJoinRequestsModal<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      { name: "getAddGroupList", data: { user_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const list = (payload.get("data") as Array<Record<string, string>> | null) || [];
        if (list.length === 0) {
          showToast("No pending join requests", "info");
          return;
        }
        this.updater.set({ joinRequestList: list }).digest();
        (document.getElementById("join-requests-modal") as HTMLDialogElement)?.showModal();
      },
    );
  },
  "approveJoinRequest<click>"(e: Record<string, unknown>) {
    const applyId = (e.params as Record<string, string>).id;
    this.srv!.save(
      { name: "passContactApply", data: { apply_id: applyId } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Approved", "success");
          const list = (
            this.updater.get("joinRequestList") as Array<Record<string, string>>
          ).filter((r) => r.apply_id !== applyId);
          this.updater.set({ joinRequestList: list }).digest();
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },
  "rejectJoinRequest<click>"(e: Record<string, unknown>) {
    const applyId = (e.params as Record<string, string>).id;
    this.srv!.save(
      { name: "refuseContactApply", data: { apply_id: applyId } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Rejected", "success");
          const list = (
            this.updater.get("joinRequestList") as Array<Record<string, string>>
          ).filter((r) => r.apply_id !== applyId);
          this.updater.set({ joinRequestList: list }).digest();
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },
  "closeJoinRequestsModal<click>"() {
    (document.getElementById("join-requests-modal") as HTMLDialogElement)?.close();
  },
});
