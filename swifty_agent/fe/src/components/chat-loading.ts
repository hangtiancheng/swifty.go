import { LitElement, html, css } from "lit";
import { customElement, property, query } from "lit/decorators.js";

@customElement("chat-loading")
export class ChatLoading extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: Boolean })
  open = false;

  @query("dialog")
  private _dialog!: HTMLDialogElement;

  override updated() {
    if (!this._dialog) return;
    if (this.open && !this._dialog.open) {
      this._dialog.showModal();
    } else if (!this.open && this._dialog.open) {
      this._dialog.close();
    }
  }

  override render() {
    return html`
      <dialog class="modal">
        <div class="modal-box flex flex-col items-center gap-4 py-10">
          <span class="loading loading-spinner loading-lg text-primary"></span>
          <p class="text-base-content text-lg font-semibold">
            AI analysis in progress
          </p>
          <p class="text-base-content/60 text-sm">Please wait...</p>
        </div>
        <form method="dialog" class="modal-backdrop">
          <button>close</button>
        </form>
      </dialog>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "chat-loading": ChatLoading;
  }
}
