import React from "react";
import { createComponent } from "@lit/react";
import { MessageBubble } from "./message-bubble";

export const MessageBubbleComponent = createComponent({
  tagName: "message-bubble",
  elementClass: MessageBubble,
  react: React,
});
