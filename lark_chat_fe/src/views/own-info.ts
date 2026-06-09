import { defineView } from "@lark.js/mvc";
import template from "./own-info.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import { isValidEmail } from "@/utils/validate";
import { showToast } from "@/utils/toast";

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,
  editData: {
    nickname: "",
    email: "",
    birthday: "",
    signature: "",
    avatar: "",
  },
  avatarFile: null as File | null,

  init() {
    this.srv = new AppService();
    this.capture("srv", this.srv);
    this.editData = {
      nickname: "",
      email: "",
      birthday: "",
      signature: "",
      avatar: "",
    };
    this.avatarFile = null;

    const auth = useAuthStore();
    this.updater.set({ userInfo: auth.userInfo }).digest();
  },

  "showEditModal<click>"() {
    (document.getElementById("edit-modal") as HTMLDialogElement)?.showModal();
  },

  "closeEditModal<click>"() {
    this.closeEditModal();
  },

  "onEditNick<input>"(e: Record<string, unknown>) {
    this.editData.nickname = (e.eventTarget as HTMLInputElement).value;
  },
  "onEditEmail<input>"(e: Record<string, unknown>) {
    this.editData.email = (e.eventTarget as HTMLInputElement).value;
  },
  "onEditBirthday<input>"(e: Record<string, unknown>) {
    this.editData.birthday = (e.eventTarget as HTMLInputElement).value;
  },
  "onEditSig<input>"(e: Record<string, unknown>) {
    this.editData.signature = (e.eventTarget as HTMLInputElement).value;
  },
  "onAvatarSelect<change>"(e: Record<string, unknown>) {
    this.avatarFile = (e.eventTarget as HTMLInputElement).files?.[0] ?? null;
  },

  "saveProfile<click>"() {
    const d = this.editData;
    if (!d.nickname && !d.email && !d.birthday && !d.signature && !this.avatarFile) {
      showToast("Please modify at least one field", "warning");
      return;
    }
    if (d.nickname && (d.nickname.length < 3 || d.nickname.length > 10)) {
      showToast("Nickname must be 3-10 characters", "error");
      return;
    }
    if (d.email && !isValidEmail(d.email)) {
      showToast("Invalid email address", "error");
      return;
    }

    if (this.avatarFile) {
      const formData = new FormData();
      formData.append("file", this.avatarFile);
      this.srv!.save({ name: "uploadAvatar", data: formData }, () => {});
      d.avatar = "/static/avatars/" + this.avatarFile.name;
    }

    const uid = useAuthStore().userInfo.uuid;
    this.srv!.save(
      { name: "updateUserInfo", data: { uuid: uid, ...d } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const code = payload.get("code");
        const msg = payload.get("message") as string;
        if (code === 200) {
          showToast(msg, "success");
          const current = useAuthStore().userInfo as import("@/types").UserInfo;
          const updated: import("@/types").UserInfo = { ...current };
          if (d.nickname) updated.nickname = d.nickname;
          if (d.email) updated.email = d.email;
          if (d.birthday) updated.birthday = d.birthday;
          if (d.signature) updated.signature = d.signature;
          if (d.avatar) updated.avatar = d.avatar;
          useAuthStore().setUserInfo(updated);
          this.closeEditModal();
        } else {
          showToast(msg, "error");
        }
      },
    );
  },

  closeEditModal() {
    (document.getElementById("edit-modal") as HTMLDialogElement)?.close();
    this.editData = {
      nickname: "",
      email: "",
      birthday: "",
      signature: "",
      avatar: "",
    };
    this.avatarFile = null;
  },
});
