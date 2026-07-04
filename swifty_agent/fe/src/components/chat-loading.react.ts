import React from "react";
import { createComponent } from "@lit/react";
import { ChatLoading } from "./chat-loading";

export const ChatLoadingComponent = createComponent({
  tagName: "chat-loading",
  elementClass: ChatLoading,
  react: React,
});
