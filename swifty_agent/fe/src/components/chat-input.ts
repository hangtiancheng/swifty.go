import { LitElement, html, css } from "lit";
import { customElement, property, state, query } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { icons } from "../icons";

@customElement("chat-input")
export class ChatInput extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: Boolean })
  isStreaming = false;

  @property({ type: String })
  currentMode: "quick" | "stream" = "quick";

  @state()
  private _toolsMenuOpen = false;

  @query("#messageInput")
  private _input!: HTMLInputElement;

  @property({ type: String, attribute: "icon-paperclip" })
  iconPaperclip = icons.paperclip;

  @property({ type: String, attribute: "icon-chevron-down" })
  iconChevronDown = icons.chevronDown;

  @property({ type: String, attribute: "icon-send" })
  iconSend = icons.sendHorizontal;

  override render() {
    const currentModeLabel = this.currentMode === "quick" ? "Quick" : "Stream";

    return html`
      <div class="shrink-0 px-4 pt-2 pb-4">
        <div class="mx-auto max-w-3xl">
          <div
            class="bg-base-200/80 border-base-300/50 focus-within:border-primary/40 flex items-end gap-2 rounded-2xl border p-2.5 shadow-sm transition-all duration-200 focus-within:shadow-md"
          >
            <!-- Tools dropdown -->
            <div class="relative shrink-0">
              <button
                class="btn btn-ghost btn-sm btn-circle text-base-content/50 hover:text-base-content"
                @click=${this._toggleToolsMenu}
              >
                <span class="h-5 w-5 [&>svg]:h-full [&>svg]:w-full"
                  >${unsafeHTML(this.iconPaperclip)}</span
                >
              </button>
              <div
                class="${this._toolsMenuOpen ? "" : "hidden"} absolute bottom-full left-0 mb-2"
              >
                <ul
                  class="menu menu-sm bg-base-100 rounded-box border-base-300 w-48 border p-1 shadow-lg"
                >
                  <li>
                    <label class="flex cursor-pointer items-center gap-3">
                      <span
                        class="h-4 w-4 shrink-0 [&>svg]:h-full [&>svg]:w-full"
                        >${unsafeHTML(this.iconPaperclip)}</span
                      >
                      <span>Upload File</span>
                      <input
                        type="file"
                        accept=".txt,.md,.markdown"
                        class="hidden"
                        @change=${this._handleFileSelected}
                      />
                    </label>
                  </li>
                </ul>
              </div>
            </div>

            <!-- Text input -->
            <input
              type="text"
              id="messageInput"
              placeholder="Ask Swifty Agent..."
              maxlength="1000"
              class="input input-ghost flex-1! border-0! bg-transparent! px-2! text-sm shadow-none! outline-none!"
              @keypress=${this._handleInputKeypress}
            />

            <!-- Right controls -->
            <div class="flex shrink-0 items-center gap-1">
              <!-- Mode selector -->
              <details class="dropdown dropdown-top dropdown-end">
                <summary
                  class="btn btn-ghost btn-xs text-base-content/50 list-none gap-1 font-normal"
                >
                  <span>${currentModeLabel}</span>
                  <span class="h-3 w-3 [&>svg]:h-full [&>svg]:w-full"
                    >${unsafeHTML(this.iconChevronDown)}</span
                  >
                </summary>
                <ul
                  class="dropdown-content menu bg-base-100 rounded-box border-base-300 mb-2 w-44 border p-1 shadow-lg"
                >
                  <li class="menu-title">
                    <span class="text-xs">Chat mode</span>
                  </li>
                  <li>
                    <a
                      class="${this.currentMode === "quick" ? "active" : ""}"
                      @click=${() => this._handleSelectMode("quick")}
                    >
                      <span>Quick</span>
                      <span class="text-base-content/40 text-xs"
                        >Instant reply</span
                      >
                    </a>
                  </li>
                  <li>
                    <a
                      class="${this.currentMode === "stream" ? "active" : ""}"
                      @click=${() => this._handleSelectMode("stream")}
                    >
                      <span>Stream</span>
                      <span class="text-base-content/40 text-xs"
                        >Live response</span
                      >
                    </a>
                  </li>
                </ul>
              </details>

              <!-- Send button -->
              <button
                class="btn btn-primary btn-sm btn-circle ${this.isStreaming ? "btn-disabled" : ""}"
                @click=${this._handleSendMessage}
                ?disabled=${this.isStreaming}
              >
                <span class="h-4 w-4 [&>svg]:h-full [&>svg]:w-full"
                  >${unsafeHTML(this.iconSend)}</span
                >
              </button>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  private _toggleToolsMenu() {
    this._toolsMenuOpen = !this._toolsMenuOpen;
  }

  private _handleInputKeypress(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      this._handleSendMessage();
    }
  }

  private _handleSendMessage() {
    if (!this._input) return;
    const content = this._input.value.trim();
    if (!content) return;

    this.dispatchEvent(
      new CustomEvent("send-message", {
        detail: { content },
        bubbles: true,
        composed: true,
      }),
    );
    this._input.value = "";
  }

  private _handleFileSelected(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    const allowedExt = [".txt", ".md", ".markdown"];
    const ext = file.name.toLowerCase().substring(file.name.lastIndexOf("."));
    if (!allowedExt.includes(ext)) {
      this._showNotification("Only .txt and .md files are supported.", "error");
      input.value = "";
      return;
    }

    const MAX_FILE_SIZE = 50 * 1024 * 1024;
    if (file.size > MAX_FILE_SIZE) {
      this._showNotification("File size cannot exceed 50 MB.", "error");
      input.value = "";
      return;
    }

    this._toolsMenuOpen = false;
    this.dispatchEvent(
      new CustomEvent("file-selected", {
        detail: { file },
        bubbles: true,
        composed: true,
      }),
    );
    input.value = "";
  }

  private _handleSelectMode(mode: "quick" | "stream") {
    this.dispatchEvent(
      new CustomEvent("mode-change", {
        detail: { mode },
        bubbles: true,
        composed: true,
      }),
    );
    this._showNotification(`Switched to ${mode} mode.`, "info");
  }

  private _showNotification(
    message: string,
    type: "info" | "success" | "warning" | "error" = "info",
  ) {
    const alertClass: Record<string, string> = {
      info: "alert-info",
      success: "alert-success",
      warning: "alert-warning",
      error: "alert-error",
    };

    const container = document.createElement("div");
    container.className = "toast toast-end toast-top z-50";

    const alertEl = document.createElement("div");
    alertEl.setAttribute("role", "alert");
    alertEl.className = `alert ${alertClass[type] || alertClass.info} shadow-lg`;

    const span = document.createElement("span");
    span.textContent = message;
    alertEl.appendChild(span);

    container.appendChild(alertEl);
    document.body.appendChild(container);

    setTimeout(() => container.remove(), 3000);
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-input": ChatInput;
  }
}
