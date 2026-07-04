import React from "react";
import { createComponent } from "@lit/react";
import { ChatWelcome } from "./chat-welcome";

export const ChatWelcomeComponent = createComponent({
  tagName: "chat-welcome",
  elementClass: ChatWelcome,
  react: React,
});
