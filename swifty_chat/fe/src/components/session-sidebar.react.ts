import React from "react";
import { createComponent, type EventName } from "@lit/react";
import { SessionSidebar } from "./session-sidebar";

export const SessionSidebarComponent = createComponent({
  tagName: "session-sidebar",
  elementClass: SessionSidebar,
  react: React,
  events: {
    onChat: "chat" as EventName<CustomEvent<string>>,
  },
});
