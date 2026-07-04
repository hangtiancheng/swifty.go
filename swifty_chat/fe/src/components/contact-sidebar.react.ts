import React from "react";
import { createComponent, type EventName } from "@lit/react";
import { ContactSidebar } from "./contact-sidebar";

export const ContactSidebarComponent = createComponent({
  tagName: "contact-sidebar",
  elementClass: ContactSidebar,
  react: React,
  events: {
    onNavigate: "navigate" as EventName<CustomEvent<string>>,
  },
});
