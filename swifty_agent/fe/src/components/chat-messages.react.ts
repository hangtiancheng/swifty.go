import React from "react";
import { createComponent } from "@lit/react";
import { ChatMessages } from "./chat-messages";

export const ChatMessagesComponent = createComponent({
  tagName: "chat-messages",
  elementClass: ChatMessages,
  react: React,
});
