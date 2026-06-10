import { Router, defineView } from "@lark.js/mvc";
import template from "./nav-bar.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import useWsStore from "@/store/ws";
import { icons } from "@/icons";

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,

  init() {
    this.srv = new AppService();
    this.capture("srv", this.srv);

    const auth = useAuthStore();
    this.updater
      .set({
        icons,
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
    this.srv!.save({ name: "wsLogout", data: { owner_id: uid } }, () => {
      useWsStore().disconnect();
      useAuthStore().clearUserInfo();
      Router.to("/login");
    });
  },
});
