import { Framework, Router, registerViewClass } from "@lark.js/mvc";
import type { FrameworkConfig } from "@lark.js/mvc";
import "./styles.css";
import "@/service/endpoints";

import useAuthStore from "@/store/auth";
import useWsStore from "@/store/ws";

import LoginView from "@/views/login";
import RegisterView from "@/views/register";
import SessionListView from "@/views/session-list";
import ContactListView from "@/views/contact-list";
import ChatView from "@/views/chat";
import OwnInfoView from "@/views/own-info";
import ManagerView from "@/views/manager";
import DashboardView from "@/views/dashboard";
import NotFoundView from "@/views/not-found";

import NavBarView from "@/components/nav-bar";
import SessionSidebarView from "@/components/session-sidebar";
import ContactSidebarView from "@/components/contact-sidebar";
import MessageBubbleView from "@/components/message-bubble";
import VideoCallView from "@/components/video-call";

registerViewClass("login", LoginView);
registerViewClass("register", RegisterView);
registerViewClass("session-list", SessionListView);
registerViewClass("contact-list", ContactListView);
registerViewClass("chat", ChatView);
registerViewClass("own-info", OwnInfoView);
registerViewClass("manager", ManagerView);
registerViewClass("dashboard", DashboardView);
registerViewClass("not-found", NotFoundView);

registerViewClass("components/nav-bar", NavBarView);
registerViewClass("components/session-sidebar", SessionSidebarView);
registerViewClass("components/contact-sidebar", ContactSidebarView);
registerViewClass("components/message-bubble", MessageBubbleView);
registerViewClass("components/video-call", VideoCallView);

Router.beforeEach(async (to) => {
  const auth = useAuthStore();
  const publicPaths = ["/login", "/register", "/dashboard"];
  if (!auth.userInfo.uuid && !publicPaths.includes(to.path ?? "")) {
    Router.to("/login", {}, true);
    return false;
  }
  return true;
});

const config: FrameworkConfig = {
  rootId: "app",
  routeMode: "history",
  defaultPath: "/login",
  defaultView: "login",
  routes: {
    "/login": "login",
    "/register": "register",
    "/chat/sessions": "session-list",
    "/chat/contacts": "contact-list",
    "/chat": "chat",
    "/chat/profile": "own-info",
    "/manager": "manager",
    "/dashboard": "dashboard",
  },
  unmatchedView: "not-found",
  error(e: Error) {
    console.error("Lark error:", e);
  },
};

Framework.boot(config);

const auth = useAuthStore();
if (auth.userInfo.uuid) {
  useWsStore().connect(auth.userInfo.uuid);
}
