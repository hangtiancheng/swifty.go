import React from "react";
import { createComponent, type EventName } from "@lit/react";
import { NavBar } from "./nav-bar";

export const NavBarComponent = createComponent({
  tagName: "nav-bar",
  elementClass: NavBar,
  react: React,
  events: {
    onNavigate: "navigate" as EventName<CustomEvent<string>>,
    onLogout: "logout" as EventName<CustomEvent>,
  },
});
