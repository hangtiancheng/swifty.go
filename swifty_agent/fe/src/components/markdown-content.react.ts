import React from "react";
import { createComponent } from "@lit/react";
import { MarkdownContent } from "./markdown-content";

export const MarkdownContentComponent = createComponent({
  tagName: "markdown-content",
  elementClass: MarkdownContent,
  react: React,
});
