import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { BASE_URL } from "../config";
import type { Message } from "../types";

@customElement("message-bubble")
export class MessageBubble extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: Array }) messageList: Message[] = [];
  @property({ type: String }) currentUserId = "";
  @property({ type: String }) currentUserAvatar = "";
  @property({ type: String }) currentUserName = "";

  private downloadFile(url: string, name: string) {
    const fileUrl = url
      ? url.startsWith("http")
        ? url
        : BASE_URL + url
      : BASE_URL + "/static/files/" + name;
    const saveName = name || "download";
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
        /* swallow: browser already logs the failed request */
      });
  }

  override render() {
    return html`
      <div>
        ${this.messageList.map((msg) => {
          const isSelf = msg.send_id === this.currentUserId;

          // Received text message
          if (!isSelf && msg.type === 0) {
            return html`
              <div class="chat chat-start gap-1">
                <div class="chat-image avatar">
                  <div
                    class="ring-base-300 ring-offset-base-100 w-10 rounded-full ring ring-offset-1"
                  >
                    <img src=${msg.send_avatar} />
                  </div>
                </div>
                <div class="chat-header text-base-content/60 text-xs">
                  ${msg.send_name}
                  <time class="ml-1 opacity-60">${msg.created_at}</time>
                </div>
                <div class="chat-bubble bg-base-100 shadow-sm">
                  ${msg.content}
                </div>
              </div>
            `;
          }

          // Received file message
          if (!isSelf && msg.type === 2) {
            return html`
              <div class="chat chat-start gap-1">
                <div class="chat-image avatar">
                  <div
                    class="ring-base-300 ring-offset-base-100 w-10 rounded-full ring ring-offset-1"
                  >
                    <img src=${msg.send_avatar} />
                  </div>
                </div>
                <div class="chat-header text-base-content/60 text-xs">
                  ${msg.send_name}
                  <time class="ml-1 opacity-60">${msg.created_at}</time>
                </div>
                <div class="chat-bubble bg-base-100 shadow-sm">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-medium">${msg.file_name}</span>
                    <span class="badge badge-sm badge-ghost text-xs"
                      >${msg.file_size}</span
                    >
                  </div>
                  <button
                    class="btn btn-sm btn-soft btn-accent mt-2 font-normal"
                    @click=${() => this.downloadFile(msg.url, msg.file_name)}
                  >
                    Download
                  </button>
                </div>
              </div>
            `;
          }

          // Sent text message
          if (isSelf && msg.type === 0) {
            return html`
              <div class="chat chat-end gap-1">
                <div class="chat-image avatar">
                  <div
                    class="ring-primary/30 ring-offset-base-100 w-10 rounded-full ring ring-offset-1"
                  >
                    <img src=${this.currentUserAvatar} />
                  </div>
                </div>
                <div class="chat-header text-base-content/60 text-xs">
                  ${this.currentUserName}
                  <time class="ml-1 opacity-60">${msg.created_at}</time>
                </div>
                <div class="chat-bubble chat-bubble-primary shadow-sm">
                  ${msg.content}
                </div>
              </div>
            `;
          }

          // Sent file message
          if (isSelf && msg.type === 2) {
            return html`
              <div class="chat chat-end gap-1">
                <div class="chat-image avatar">
                  <div
                    class="ring-primary/30 ring-offset-base-100 w-10 rounded-full ring ring-offset-1"
                  >
                    <img src=${this.currentUserAvatar} />
                  </div>
                </div>
                <div class="chat-header text-base-content/60 text-xs">
                  ${this.currentUserName}
                  <time class="ml-1 opacity-60">${msg.created_at}</time>
                </div>
                <div class="chat-bubble chat-bubble-primary shadow-sm">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-medium">${msg.file_name}</span>
                    <span class="badge badge-sm badge-ghost text-xs"
                      >${msg.file_size}</span
                    >
                  </div>
                  <div class="mt-1 text-xs opacity-70">Sent</div>
                </div>
              </div>
            `;
          }

          return html``;
        })}
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "message-bubble": MessageBubble;
  }
}
