import { Router, defineView } from "@lark.js/mvc";
import template from "./session-sidebar.html";
import AppService from "@/service/index";
import "@/service/endpoints";
import useAuthStore from "@/store/auth";
import useSessionStore from "@/store/session";
import { resolveAvatar } from "@/utils/avatar";
import type { SessionItem } from "@/types";

export default defineView({
  template,
  srv: null as InstanceType<typeof AppService> | null,

  init() {
    this.updater.set({ userSessions: [], groupSessions: [] }).digest();
    this.srv = new AppService();
    this.capture("srv", this.srv);
  },

  "toggleUsers<change>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.all(
      { name: "getUserSessionList", data: { owner_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const list = (payload.get("data") as SessionItem[] | null) || [];
        list.forEach((u) => {
          u.avatar = resolveAvatar(u.avatar);
        });
        useSessionStore().setUserSessions(list);
        this.updater.set({ userSessions: list }).digest();
      },
    );
  },

  "toggleGroups<change>"() {
    const uid = useAuthStore().userInfo.uuid;
    this.srv!.all(
      { name: "getGroupSessionList", data: { owner_id: uid } },
      (_errors: unknown[], payload: { get: (k: string) => unknown }) => {
        const list = (payload.get("data") as SessionItem[] | null) || [];
        list.forEach((g) => {
          g.avatar = resolveAvatar(g.avatar);
        });
        useSessionStore().setGroupSessions(list);
        this.updater.set({ groupSessions: list }).digest();
      },
    );
  },

  "chatUser<click>"(e: Record<string, unknown>) {
    const params = e.params as Record<string, string>;
    Router.to("/chat", { id: params.id });
  },

  "chatGroup<click>"(e: Record<string, unknown>) {
    const params = e.params as Record<string, string>;
    Router.to("/chat", { id: params.id });
  },

  "onSearch<input>"() {},
});
