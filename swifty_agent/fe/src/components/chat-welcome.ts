import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { icons } from "../icons";

@customElement("chat-welcome")
export class ChatWelcome extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: String, attribute: "icon-sparkles" })
  iconSparkles = icons.sparkles;

  override render() {
    return html`
      <div class="mb-12 px-6 text-center">
        <div
          class="from-primary to-secondary mx-auto mb-6 flex h-16 w-16 items-center justify-center rounded-2xl bg-linear-to-br shadow-lg"
        >
          <span
            class="text-primary-content h-8 w-8 [&>svg]:h-full [&>svg]:w-full"
            >${unsafeHTML(this.iconSparkles)}</span
          >
        </div>
        <h2 class="text-base-content mb-2 text-2xl font-semibold">
          Welcome to Swifty Agent
        </h2>
        <p
          class="text-base-content/50 mx-auto max-w-md text-sm leading-relaxed"
        >
          Your intelligent operations assistant. Upload project docs via the
          tools menu, then start chatting.
        </p>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-welcome": ChatWelcome;
  }
}
