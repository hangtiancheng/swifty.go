import { Router, defineView } from "@lark.js/mvc";
import template from "./nav-bar.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import useWsStore from "@/store/ws";

export default defineView({
  template,
  svc: null as InstanceType<typeof AppService> | null,

  init() {
    this.svc = new AppService();
    this.capture("svc", this.svc);

    const auth = useAuthStore();
    this.updater
      .set({
        avatar: auth.userInfo.avatar,
        isAdmin: auth.userInfo.is_admin === 1,
      })
      .digest();
  },

  "goSessions<click>"() {
    Router.to("/chat/sessions");
  },
  "goContacts<click>"() {
    Router.to("/chat/contacts");
  },
  "goProfile<click>"() {
    Router.to("/chat/profile");
  },
  "goManager<click>"() {
    Router.to("/manager");
  },

  "logout<click>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.svc!.save({ name: "wsLogout", data: { owner_id: uid } }, () => {
      useWsStore().disconnect();
      useAuthStore().clearUserInfo();
      Router.to("/login");
    });
  },
});
