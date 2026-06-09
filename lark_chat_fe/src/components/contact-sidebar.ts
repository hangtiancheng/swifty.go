import { Router, defineView } from "@lark.js/mvc";
import template from "./contact-sidebar.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import { showToast } from "@/utils/toast";

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,
  applyId: "",
  applyMsg: "",
  groupName: "",

  init() {
    this.updater
      .set({
        friendList: [],
        myGroupList: [],
        joinedGroupList: [],
        requestList: [],
      })
      .digest();
    this.srv = new AppService();
    this.capture("srv", this.srv);
  },

  "toggleFriends<change>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.all(
      { name: "getUserList", data: { owner_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        this.updater.set({ friendList: payload.get("data") || [] }).digest();
      },
    );
  },

  "toggleMyGroups<change>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.all(
      { name: "loadMyGroup", data: { owner_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        this.updater.set({ myGroupList: payload.get("data") || [] }).digest();
      },
    );
  },

  "toggleJoinedGroups<change>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.all(
      { name: "loadMyJoinedGroup", data: { owner_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        this.updater.set({ joinedGroupList: payload.get("data") || [] }).digest();
      },
    );
  },

  "chatUser<click>"(e: Record<string, unknown>) {
    const contactId = (e.params as Record<string, string>).id;
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      {
        name: "checkOpenSessionAllowed",
        data: { send_id: uid, receive_id: contactId },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200 && payload.get("data") === true) {
          Router.to("/chat", { id: contactId });
        } else {
          showToast((payload.get("message") as string) || "Cannot open session", "warning");
        }
      },
    );
  },

  "chatGroup<click>"(e: Record<string, unknown>) {
    const contactId = (e.params as Record<string, string>).id;
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      {
        name: "checkOpenSessionAllowed",
        data: { send_id: uid, receive_id: contactId },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200 && payload.get("data") === true) {
          Router.to("/chat", { id: contactId });
        } else {
          showToast((payload.get("message") as string) || "Cannot open session", "warning");
        }
      },
    );
  },

  // -- Apply Contact / Group --
  "showApplyModal<click>"() {
    this.applyId = "";
    this.applyMsg = "";
    (document.getElementById("apply-modal") as HTMLDialogElement)?.showModal();
  },
  "closeApplyModal<click>"() {
    (document.getElementById("apply-modal") as HTMLDialogElement)?.close();
  },
  "onApplyIdInput<input>"(e: Record<string, unknown>) {
    this.applyId = (e.eventTarget as HTMLInputElement).value;
  },
  "onApplyMsgInput<input>"(e: Record<string, unknown>) {
    this.applyMsg = (e.eventTarget as HTMLTextAreaElement).value;
  },
  "submitApply<click>"() {
    if (!this.applyId) {
      showToast("Please enter an ID", "error");
      return;
    }
    const uid = useAuthStore().userInfo.uuid;
    const isGroup = this.applyId.startsWith("G");

    if (isGroup) {
      this.srv!.save(
        { name: "checkGroupAddMode", data: { group_id: this.applyId } },
        (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
          if (payload.get("code") === 200 && payload.get("data") === 0) {
            this.srv!.save(
              {
                name: "enterGroupDirectly",
                data: { user_id: uid, group_id: this.applyId },
              },
              (_e2: unknown[], p2: { get: (k: string) => unknown }) => {
                if (p2.get("code") === 200) {
                  showToast("Joined group", "success");
                  (document.getElementById("apply-modal") as HTMLDialogElement)?.close();
                } else showToast(p2.get("message") as string, "error");
              },
            );
          } else {
            this.srv!.save(
              {
                name: "applyContact",
                data: {
                  user_id: uid,
                  contact_id: this.applyId,
                  contact_type: 1,
                  message: this.applyMsg,
                },
              },
              (_e2: unknown[], p2: { get: (k: string) => unknown }) => {
                if (p2.get("code") === 200) {
                  showToast("Application sent", "success");
                  (document.getElementById("apply-modal") as HTMLDialogElement)?.close();
                } else showToast(p2.get("message") as string, "error");
              },
            );
          }
        },
      );
    } else {
      this.srv!.save(
        {
          name: "applyContact",
          data: {
            user_id: uid,
            contact_id: this.applyId,
            contact_type: 0,
            message: this.applyMsg,
          },
        },
        (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
          if (payload.get("code") === 200) {
            showToast("Application sent", "success");
            (document.getElementById("apply-modal") as HTMLDialogElement)?.close();
          } else showToast(payload.get("message") as string, "error");
        },
      );
    }
  },

  // -- Create Group --
  "showCreateGroupModal<click>"() {
    this.groupName = "";
    (document.getElementById("create-group-modal") as HTMLDialogElement)?.showModal();
  },
  "closeCreateGroupModal<click>"() {
    (document.getElementById("create-group-modal") as HTMLDialogElement)?.close();
  },
  "onGroupNameInput<input>"(e: Record<string, unknown>) {
    this.groupName = (e.eventTarget as HTMLInputElement).value;
  },
  "submitCreateGroup<click>"() {
    if (!this.groupName) {
      showToast("Please enter a group name", "error");
      return;
    }
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      {
        name: "createGroup",
        data: { name: this.groupName, owner_id: uid, avatar: "" },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Group created", "success");
          (document.getElementById("create-group-modal") as HTMLDialogElement)?.close();
        } else showToast(payload.get("message") as string, "error");
      },
    );
  },

  // -- Friend Requests --
  "showNewContactModal<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      { name: "getNewContactList", data: { user_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const list = (payload.get("data") as unknown[] | null) || [];
        if (list.length === 0) {
          showToast("No pending friend requests", "info");
          return;
        }
        this.updater.set({ requestList: list }).digest();
        (document.getElementById("friend-requests-modal") as HTMLDialogElement)?.showModal();
      },
    );
  },
  "closeRequestsModal<click>"() {
    (document.getElementById("friend-requests-modal") as HTMLDialogElement)?.close();
  },
  "approveRequest<click>"(e: Record<string, unknown>) {
    const applyId = (e.params as Record<string, string>).id;
    this.srv!.save(
      { name: "passContactApply", data: { apply_id: applyId } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Approved", "success");
          this.removeRequest(applyId);
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },

  "refuseRequest<click>"(e: Record<string, unknown>) {
    const applyId = (e.params as Record<string, string>).id;
    this.srv!.save(
      { name: "refuseContactApply", data: { apply_id: applyId } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Refused", "success");
          this.removeRequest(applyId);
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },

  "blockRequest<click>"(e: Record<string, unknown>) {
    const applyId = (e.params as Record<string, string>).id;
    this.srv!.save(
      { name: "blackApply", data: { apply_id: applyId } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Blocked", "success");
          this.removeRequest(applyId);
        } else {
          showToast(payload.get("message") as string, "error");
        }
      },
    );
  },

  "unblockUser<click>"(e: Record<string, unknown>) {
    const contactId = (e.params as Record<string, string>).id;
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      {
        name: "cancelBlackContact",
        data: { user_id: uid, contact_id: contactId },
      },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        if (payload.get("code") === 200) {
          showToast("Contact unblocked", "success");
        } else {
          showToast((payload.get("message") as string) || "Failed to unblock", "error");
        }
      },
    );
  },

  removeRequest(applyId: string) {
    const list = (this.updater.get("requestList") as Array<Record<string, string>>).filter(
      (r) => r.apply_id !== applyId,
    );
    this.updater.set({ requestList: list }).digest();
  },
});
