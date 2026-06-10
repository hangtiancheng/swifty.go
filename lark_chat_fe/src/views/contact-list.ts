import View from "@/view";
import template from "./contact-list.html";
import { icons } from "@/icons";

export default View.extend({
  template,
  init() {
    this.updater.set({ icons }).digest();
  },
});
