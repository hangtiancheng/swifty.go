import { Router, defineView } from "@lark.js/mvc";
import template from "./register.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import useWsStore from "@/store/ws";
import { isValidPhone } from "@/utils/validate";
import { showToast } from "@/utils/toast";

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,

  init() {
    this.updater
      .set({ nickname: "", telephone: "", password: "", confirmPassword: "" })
      .digest();
    this.srv = new AppService();
    this.capture("srv", this.srv);
  },

  "onNickInput<input>"(e: Record<string, unknown>) {
    this.updater.set({ nickname: (e.eventTarget as HTMLInputElement).value });
  },
  "onTelInput<input>"(e: Record<string, unknown>) {
    this.updater.set({ telephone: (e.eventTarget as HTMLInputElement).value });
  },
  "onPwdInput<input>"(e: Record<string, unknown>) {
    this.updater.set({ password: (e.eventTarget as HTMLInputElement).value });
  },
  "onConfirmPwdInput<input>"(e: Record<string, unknown>) {
    this.updater.set({
      confirmPassword: (e.eventTarget as HTMLInputElement).value,
    });
  },

  "handleRegister<click>"() {
    const d = this.updater.get() as Record<string, string>;
    if (!d.nickname || !d.telephone || !d.password || !d.confirmPassword) {
      showToast("Please fill in all fields", "error");
      return;
    }
    if (d.nickname.length < 3 || d.nickname.length > 10) {
      showToast("Nickname must be 3-10 characters", "error");
      return;
    }
    if (!isValidPhone(d.telephone)) {
      showToast("Invalid phone number", "error");
      return;
    }
    if (d.password !== d.confirmPassword) {
      showToast("Passwords do not match", "error");
      return;
    }

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { confirmPassword: _, ...payload } = d;
    this.srv!.save(
      { name: "register", data: payload },
      (_errors: unknown[], rsp: { get: (k: string) => unknown }) => {
        const code = rsp?.get("code");
        const data = rsp?.get("data") as Record<string, unknown> | null;
        const msg = rsp?.get("message") as string;
        if (code === 200) {
          showToast(msg, "success");
          useAuthStore().setUserInfo(data as never);
          useWsStore().connect(data!.uuid as string);
          Router.to("/chat/sessions");
        } else {
          showToast(msg || "Registration failed", "error");
        }
      },
    );
  },

  "goLogin<click>"() {
    Router.to("/login");
  },
});
