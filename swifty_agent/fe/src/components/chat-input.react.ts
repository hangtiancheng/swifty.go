import React from "react";
import { createComponent, type EventName } from "@lit/react";
import { ChatInput } from "./chat-input";

export const ChatInputComponent = createComponent({
  tagName: "chat-input",
  elementClass: ChatInput,
  react: React,
  events: {
    onSendMessage: "send-message" as EventName<
      CustomEvent<{ content: string }>
    >,
    onFileSelected: "file-selected" as EventName<CustomEvent<{ file: File }>>,
    onModeChange: "mode-change" as EventName<
      CustomEvent<{ mode: "quick" | "stream" }>
    >,
  },
});
