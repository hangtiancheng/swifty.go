import React from "react";
import { createComponent, type EventName } from "@lit/react";
import { ChatSidebar } from "./chat-sidebar";

export const ChatSidebarComponent = createComponent({
  tagName: "chat-sidebar",
  elementClass: ChatSidebar,
  react: React,
  events: {
    onNewChat: "new-chat" as EventName<CustomEvent>,
    onLoadHistory: "load-history" as EventName<CustomEvent<{ id: string }>>,
    onDeleteHistory: "delete-history" as EventName<CustomEvent<{ id: string }>>,
  },
});
