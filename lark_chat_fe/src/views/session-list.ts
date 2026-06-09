import View from "@/view";
import template from "./session-list.html";

export default View.extend({
  template,
  init() {
    this.updater.digest();
  },
});
