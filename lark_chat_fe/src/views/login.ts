import { Router, defineView } from "@lark.js/mvc";
import template from "./login.html";
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
    this.updater.set({ telephone: "", password: "" }).digest();
    this.srv = new AppService();
    this.capture("srv", this.srv);
  },

  "onTelInput<input>"(e: Record<string, unknown>) {
    this.updater.set({ telephone: (e.eventTarget as HTMLInputElement).value });
  },

  "onPwdInput<input>"(e: Record<string, unknown>) {
    this.updater.set({ password: (e.eventTarget as HTMLInputElement).value });
  },

  "handleLogin<click>"() {
    const tel = this.updater.get("telephone") as string;
    const pwd = this.updater.get("password") as string;
    if (!tel || !pwd) {
      showToast("Please fill in all fields", "error");
      return;
    }
    if (!isValidPhone(tel)) {
      showToast("Invalid phone number", "error");
      return;
    }

    this.srv!.save(
      { name: "login", data: { telephone: tel, password: pwd } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const code = payload.get("code");
        const data = payload.get("data") as Record<string, unknown> | null;
        const msg = payload.get("message") as string;
        if (code === 200) {
          if (data && data.status === 1) {
            showToast("This account has been banned", "error");
            return;
          }
          showToast(msg, "success");
          useAuthStore().setUserInfo(data as never);
          useWsStore().connect((data as Record<string, string>).uuid);
          Router.to("/chat/sessions");
        } else {
          showToast(msg || "Login failed", "error");
        }
      },
    );
  },

  "goRegister<click>"() {
    Router.to("/register");
  },
});
