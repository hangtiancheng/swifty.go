import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { icons } from "../icons";

@customElement("chat-topbar")
export class ChatTopbar extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: String, attribute: "icon-layers" })
  iconLayers = icons.layers;

  override render() {
    return html`
      <div
        class="border-base-200 flex shrink-0 items-center justify-end border-b px-5 py-3"
      >
        <button
          class="btn btn-warning btn-sm gap-2 rounded-full font-medium shadow-sm"
          @click=${this._handleTriggerAIOps}
        >
          <span class="h-4 w-4 [&>svg]:h-full [&>svg]:w-full"
            >${unsafeHTML(this.iconLayers)}</span
          >
          <span>AI Ops</span>
        </button>
      </div>
    `;
  }

  private _handleTriggerAIOps() {
    this.dispatchEvent(
      new CustomEvent("trigger-aiops", { bubbles: true, composed: true }),
    );
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-topbar": ChatTopbar;
  }
}
