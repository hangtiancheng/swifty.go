import React from "react";
import { createComponent } from "@lit/react";
import { ChatMessageElement } from "./chat-message";

export const ChatMessageComponent = createComponent({
  tagName: "chat-message",
  elementClass: ChatMessageElement,
  react: React,
});
