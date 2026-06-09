import View from "@/view";
import template from "./contact-list.html";

export default View.extend({
  template,
  init() {
    this.updater.digest();
  },
});
