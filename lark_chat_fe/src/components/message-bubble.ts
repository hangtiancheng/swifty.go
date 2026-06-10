import View from "@/view";
import template from "./message-bubble.html";
import { BASE_URL } from "@/config";

export default View.extend({
  template,
  init() {
    this.updater
      .set({
        messageList: [],
        currentUserId: "",
        currentUserAvatar: "",
        currentUserName: "",
      })
      .digest();
  },

  setData(data: Record<string, unknown>) {
    this.updater.set(data).digest();
  },

  "downloadFile<click>"(e: Record<string, unknown>) {
    const params = e.params as Record<string, string>;
    const fileName = params.name;
    fetch(BASE_URL + "/static/files/" + fileName)
      .then((r) => r.blob())
      .then((blob) => {
        const link = document.createElement("a");
        link.href = URL.createObjectURL(blob);
        link.download = fileName;
        document.body.appendChild(link);
        link.click();
        link.remove();
      });
  },
});
