import { LitElement, html, css } from "lit";
import { customElement, property, query } from "lit/decorators.js";
import type { ChatMessage } from "../types";
import "./chat-message";

@customElement("chat-messages")
export class ChatMessages extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
      flex: 1;
      overflow-y: auto;
      min-height: 0;
    }
  `;

  @property({ type: Array })
  messages: ChatMessage[] = [];

  @property({ type: Boolean })
  isStreaming = false;

  @query("#chatMessagesContainer")
  private _container!: HTMLDivElement;

  override updated() {
    setTimeout(() => {
      if (this._container) {
        this._container.scrollTop = this._container.scrollHeight;
      }
    }, 60);
  }

  override render() {
    return html`
      <div
        class="min-h-0 flex-1 overflow-y-auto px-4 py-6"
        id="chatMessagesContainer"
      >
        <div class="mx-auto max-w-3xl space-y-6">
          ${this.messages.map(
            (msg) => html`
              <chat-message
                .message=${msg}
                .isStreaming=${this.isStreaming}
              ></chat-message>
            `,
          )}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-messages": ChatMessages;
  }
}
