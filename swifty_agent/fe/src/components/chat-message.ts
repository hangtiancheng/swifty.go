import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { icons } from "../icons";
import type { ChatMessage } from "../types";

@customElement("chat-message")
export class ChatMessageElement extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: Object })
  message: ChatMessage = { id: "", type: "user", content: "", timestamp: "" };

  @property({ type: Boolean })
  isStreaming = false;

  @property({ type: String, attribute: "icon-sparkles" })
  iconSparkles = icons.sparkles;

  override render() {
    const msg = this.message;

    if (msg.type === "assistant") {
      return html`
        <div class="chat chat-start gap-2">
          <div class="chat-image avatar">
            <div
              class="from-primary to-secondary flex w-8 items-center justify-center rounded-full bg-linear-to-br shadow-sm"
            >
              <span
                class="text-primary-content h-4 w-4 [&>svg]:h-full [&>svg]:w-full"
                >${unsafeHTML(this.iconSparkles)}</span
              >
            </div>
          </div>
          <div
            class="chat-bubble bg-base-200/60 text-base-content min-h-0 shadow-sm"
            id="msg-${msg.id}"
          >
            ${
              msg.details && msg.details.length > 0
                ? html`
                    <div class="collapse-arrow bg-base-200 collapse mb-2">
                      <input type="checkbox" id="details-${msg.id}" />
                      <label
                        for="details-${msg.id}"
                        class="collapse-title min-h-0 py-2 pr-8 text-sm font-medium"
                      >
                        View steps (${msg.details.length})
                      </label>
                      <div class="collapse-content space-y-2 pb-2!">
                        ${msg.details.map(
                          (detail, i) => html`
                            <div
                              class="border-primary/30 border-l-2 py-1 pl-3 text-xs"
                            >
                              <span class="text-primary font-semibold"
                                >Step ${i + 1}:</span
                              >
                              ${detail}
                            </div>
                          `,
                        )}
                      </div>
                    </div>
                  `
                : ""
            }
            <markdown-content .content=${msg.content}></markdown-content>
            ${
              this.isStreaming
                ? html`<span
                    class="bg-primary/60 ml-0.5 inline-block h-4 w-1.5 animate-pulse rounded-sm align-middle"
                  ></span>`
                : ""
            }
          </div>
        </div>
      `;
    }

    if (msg.type === "user") {
      return html`
        <div class="chat chat-end gap-2">
          <div
            class="chat-bubble chat-bubble-primary shadow-sm"
            id="msg-${msg.id}"
          >
            <div class="prose prose-sm prose-invert max-w-none">
              ${msg.content}
            </div>
          </div>
        </div>
      `;
    }

    return html``;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-message": ChatMessageElement;
  }
}
