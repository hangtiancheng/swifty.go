import React from "react";
import { createComponent, type EventName } from "@lit/react";
import { ChatTopbar } from "./chat-topbar";

export const ChatTopbarComponent = createComponent({
  tagName: "chat-topbar",
  elementClass: ChatTopbar,
  react: React,
  events: {
    onTriggerAiops: "trigger-aiops" as EventName<CustomEvent>,
  },
});
