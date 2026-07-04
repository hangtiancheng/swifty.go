import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { icons } from "../icons";
import type { ChatHistoryItem } from "../types";

@customElement("chat-sidebar")
export class ChatSidebar extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: Array })
  chatHistories: ChatHistoryItem[] = [];

  @property({ type: String })
  sessionId = "";

  @property({ type: String, attribute: "icon-sparkles" })
  iconSparkles = icons.sparkles;

  @property({ type: String, attribute: "icon-plus" })
  iconPlus = icons.plus;

  @property({ type: String, attribute: "icon-x" })
  iconX = icons.x;

  override render() {
    return html`
      <aside
        class="bg-base-200 border-base-300/60 flex h-full w-64 shrink-0 flex-col border-r"
      >
        <!-- Brand header -->
        <div class="px-5 pt-5 pb-3">
          <div class="flex items-center gap-3">
            <div
              class="bg-primary flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
            >
              <span
                class="text-primary-content h-4 w-4 [&>svg]:h-full [&>svg]:w-full"
                >${unsafeHTML(this.iconSparkles)}</span
              >
            </div>
            <div>
              <h1 class="text-base-content text-base leading-tight font-bold">
                Swifty Agent
              </h1>
              <p class="text-base-content/40 text-xs">AI Assistant</p>
            </div>
          </div>
        </div>

        <!-- New chat action -->
        <div class="px-3 pb-2">
          <button
            class="btn btn-ghost hover:bg-base-300/60 w-full justify-start gap-3 font-normal"
            @click=${this._handleNewChat}
          >
            <span class="h-5 w-5 shrink-0 [&>svg]:h-full [&>svg]:w-full"
              >${unsafeHTML(this.iconPlus)}</span
            >
            <span>New Chat</span>
          </button>
        </div>

        <div class="divider divider-horizontal mx-3 my-0! opacity-30"></div>

        <!-- Chat history -->
        <div class="flex-1 overflow-y-auto px-3 pt-3 pb-3">
          <p
            class="text-base-content/40 px-2 pb-2 text-xs font-semibold tracking-wider uppercase"
          >
            Recent
          </p>
          <ul class="menu menu-sm gap-0.5 p-0">
            ${this.chatHistories.map(
              (history) => html`
                <li>
                  <a
                    class="group flex items-center gap-0 rounded-lg py-2"
                    @click=${() => this._handleLoadHistory(history.id)}
                  >
                    <span class="flex-1 truncate text-sm"
                      >${history.title}</span
                    >
                    <button
                      class="btn btn-ghost btn-xs shrink-0 opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                      @click=${(e: Event) => this._handleDeleteHistory(e, history.id)}
                    >
                      <span class="h-3 w-3 [&>svg]:h-full [&>svg]:w-full"
                        >${unsafeHTML(this.iconX)}</span
                      >
                    </button>
                  </a>
                </li>
              `,
            )}
          </ul>
        </div>
      </aside>
    `;
  }

  private _handleNewChat() {
    this.dispatchEvent(
      new CustomEvent("new-chat", { bubbles: true, composed: true }),
    );
  }

  private _handleLoadHistory(id: string) {
    this.dispatchEvent(
      new CustomEvent("load-history", {
        detail: { id },
        bubbles: true,
        composed: true,
      }),
    );
  }

  private _handleDeleteHistory(e: Event, id: string) {
    e.stopPropagation();
    this.dispatchEvent(
      new CustomEvent("delete-history", {
        detail: { id },
        bubbles: true,
        composed: true,
      }),
    );
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-sidebar": ChatSidebar;
  }
}
