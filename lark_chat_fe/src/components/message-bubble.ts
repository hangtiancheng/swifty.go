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
    const fileUrl = params.url
      ? params.url.startsWith("http")
        ? params.url
        : BASE_URL + params.url
      : BASE_URL + "/static/files/" + params.name;
    const saveName = params.name || "download";
    fetch(fileUrl)
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.blob();
      })
      .then((blob) => {
        const link = document.createElement("a");
        link.href = URL.createObjectURL(blob);
        link.download = saveName;
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(link.href);
      })
      .catch(() => {
        // swallow: browser will already log the failed request
      });
  },
});
