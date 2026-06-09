import { Router, defineView } from "@lark.js/mvc";
import template from "./manager.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import { showToast } from "@/utils/toast";

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,
  selectedUserIds: [] as string[],
  selectedGroupIds: [] as string[],

  init() {
    this.updater.set({ currentPanel: "none", userList: [], groupList: [] }).digest();
    this.srv = new AppService();
    this.capture("srv", this.srv);
    this.selectedUserIds = [];
    this.selectedGroupIds = [];
  },

  "showPanel<click>"(e: Record<string, unknown>) {
    const panel = (e.params as Record<string, string>).panel;
    this.selectedUserIds = [];
    this.selectedGroupIds = [];
    this.updater.set({ currentPanel: panel });

    if (panel === "disable-user" || panel === "delete-user" || panel === "set-admin") {
      this.loadUserList();
    } else if (panel === "disable-group" || panel === "delete-group") {
      this.loadGroupList();
    } else {
      this.updater.digest();
    }
  },

  loadUserList() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      { name: "getUserInfoList", data: { owner_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        this.updater.set({ userList: payload.get("data") || [] }).digest();
      },
    );
  },

  loadGroupList() {
    this.srv!.save(
      { name: "getGroupInfoList", data: {} },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        this.updater.set({ groupList: payload.get("data") || [] }).digest();
      },
    );
  },

  "toggleUser<change>"(e: Record<string, unknown>) {
    const uuid = (e.params as Record<string, string>).uuid;
    const checked = (e.eventTarget as HTMLInputElement).checked;
    if (checked) this.selectedUserIds.push(uuid);
    else this.selectedUserIds = this.selectedUserIds.filter((id: string) => id !== uuid);
  },

  "toggleAllUsers<change>"(e: Record<string, unknown>) {
    const checked = (e.eventTarget as HTMLInputElement).checked;
    const list = this.updater.get("userList") as Array<Record<string, string>>;
    this.selectedUserIds = checked ? list.map((u) => u.uuid) : [];
  },

  "toggleGroup<change>"(e: Record<string, unknown>) {
    const uuid = (e.params as Record<string, string>).uuid;
    const checked = (e.eventTarget as HTMLInputElement).checked;
    if (checked) this.selectedGroupIds.push(uuid);
    else this.selectedGroupIds = this.selectedGroupIds.filter((id: string) => id !== uuid);
  },

  "toggleAllGroups<change>"(e: Record<string, unknown>) {
    const checked = (e.eventTarget as HTMLInputElement).checked;
    const list = this.updater.get("groupList") as Array<Record<string, string>>;
    this.selectedGroupIds = checked ? list.map((g) => g.uuid) : [];
  },

  "enableSelectedUsers<click>"() {
    if (!this.selectedUserIds.length) {
      showToast("No users selected", "warning");
      return;
    }
    this.srv!.save({ name: "ableUsers", data: { uuid_list: this.selectedUserIds } }, () => {
      showToast("Users enabled", "success");
      this.loadUserList();
    });
  },

  "disableSelectedUsers<click>"() {
    if (!this.selectedUserIds.length) {
      showToast("No users selected", "warning");
      return;
    }
    this.srv!.save({ name: "disableUsers", data: { uuid_list: this.selectedUserIds } }, () => {
      showToast("Users disabled", "success");
      this.loadUserList();
    });
  },

  "deleteSelectedUsers<click>"() {
    if (!this.selectedUserIds.length) {
      showToast("No users selected", "warning");
      return;
    }
    this.srv!.save({ name: "deleteUsers", data: { uuid_list: this.selectedUserIds } }, () => {
      showToast("Users deleted", "success");
      this.loadUserList();
    });
  },

  "setAdminSelected<click>"(e: Record<string, unknown>) {
    if (!this.selectedUserIds.length) {
      showToast("No users selected", "warning");
      return;
    }
    const isAdmin = Number((e.params as Record<string, string>).val);
    this.srv!.save(
      {
        name: "setAdmin",
        data: { uuid_list: this.selectedUserIds, is_admin: isAdmin },
      },
      () => {
        showToast(isAdmin ? "Admin granted" : "Admin revoked", "success");
        this.loadUserList();
      },
    );
  },

  "enableSelectedGroups<click>"() {
    if (!this.selectedGroupIds.length) {
      showToast("No groups selected", "warning");
      return;
    }
    this.srv!.save(
      {
        name: "setGroupsStatus",
        data: { uuid_list: this.selectedGroupIds, status: 0 },
      },
      () => {
        showToast("Groups enabled", "success");
        this.loadGroupList();
      },
    );
  },

  "disableSelectedGroups<click>"() {
    if (!this.selectedGroupIds.length) {
      showToast("No groups selected", "warning");
      return;
    }
    this.srv!.save(
      {
        name: "setGroupsStatus",
        data: { uuid_list: this.selectedGroupIds, status: 1 },
      },
      () => {
        showToast("Groups disabled", "success");
        this.loadGroupList();
      },
    );
  },

  "deleteSelectedGroups<click>"() {
    if (!this.selectedGroupIds.length) {
      showToast("No groups selected", "warning");
      return;
    }
    this.srv!.save({ name: "deleteGroups", data: { uuid_list: this.selectedGroupIds } }, () => {
      showToast("Groups deleted", "success");
      this.loadGroupList();
    });
  },

  "backToChat<click>"() {
    Router.to("/chat/sessions");
  },
});
