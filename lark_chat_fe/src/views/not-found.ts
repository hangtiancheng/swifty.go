import { Router } from "@lark.js/mvc";
import View from "@/view";
import template from "./not-found.html";

export default View.extend({
  template,
  init() {
    this.updater.digest();
  },
  "goHome<click>"() {
    Router.to("/login");
  },
});
