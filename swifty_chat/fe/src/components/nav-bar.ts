import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { MessageSquare, Users, User, Settings, LogOut } from "lucide-react";
import { iconToSvg } from "../utils/icon";

@customElement("nav-bar")
export class NavBar extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static styles = css`
    :host {
      display: block;
      height: 100%;
    }
  `;

  @property({ type: String }) avatar = "";
  @property({ type: Boolean }) isAdmin = false;

  render() {
    return html`
      <div
        class="border-base-300 bg-base-200 flex h-full w-16 flex-col items-center border-r py-4"
      >
        <div class="mb-6">
          ${
            this.avatar
              ? html`
                  <div class="avatar">
                    <div
                      class="ring-primary/30 ring-offset-base-100 w-10 rounded-full ring ring-offset-1"
                    >
                      <img src="${this.avatar}" />
                    </div>
                  </div>
                `
              : ""
          }
        </div>

        <div class="flex flex-1 flex-col gap-3">
          <div class="tooltip tooltip-right" data-tip="Sessions">
            <button
              class="btn btn-ghost btn-sm btn-square text-base-content/70 hover:bg-base-300 font-normal"
              @click=${() =>
                this.dispatchEvent(
                  new CustomEvent("navigate", { detail: "/chat/sessions" }),
                )}
            >
              <span class="h-5 w-5 [&>svg]:h-full [&>svg]:w-full"
                >${unsafeHTML(iconToSvg(MessageSquare))}</span
              >
            </button>
          </div>
          <div class="tooltip tooltip-right" data-tip="Contacts">
            <button
              class="btn btn-ghost btn-sm btn-square text-base-content/70 hover:bg-base-300 font-normal"
              @click=${() =>
                this.dispatchEvent(
                  new CustomEvent("navigate", { detail: "/chat/contacts" }),
                )}
            >
              <span class="h-5 w-5 [&>svg]:h-full [&>svg]:w-full"
                >${unsafeHTML(iconToSvg(Users))}</span
              >
            </button>
          </div>
          <div class="tooltip tooltip-right" data-tip="Profile">
            <button
              class="btn btn-ghost btn-sm btn-square text-base-content/70 hover:bg-base-300 font-normal"
              @click=${() =>
                this.dispatchEvent(
                  new CustomEvent("navigate", { detail: "/chat/profile" }),
                )}
            >
              <span class="h-5 w-5 [&>svg]:h-full [&>svg]:w-full"
                >${unsafeHTML(iconToSvg(User))}</span
              >
            </button>
          </div>
        </div>

        <div class="flex flex-col gap-2">
          ${
            this.isAdmin
              ? html`
                  <div class="tooltip tooltip-right" data-tip="Admin">
                    <button
                      class="btn btn-ghost btn-sm btn-square text-base-content/70 hover:bg-base-300 font-normal"
                      @click=${() =>
                        this.dispatchEvent(
                          new CustomEvent("navigate", { detail: "/manager" }),
                        )}
                    >
                      <span class="h-5 w-5 [&>svg]:h-full [&>svg]:w-full"
                        >${unsafeHTML(iconToSvg(Settings))}</span
                      >
                    </button>
                  </div>
                `
              : ""
          }
          <div class="tooltip tooltip-right" data-tip="Sign Out">
            <button
              class="btn btn-ghost btn-sm btn-square text-error hover:bg-error/10 font-normal"
              @click=${() => this.dispatchEvent(new CustomEvent("logout"))}
            >
              <span class="h-5 w-5 [&>svg]:h-full [&>svg]:w-full"
                >${unsafeHTML(iconToSvg(LogOut))}</span
              >
            </button>
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "nav-bar": NavBar;
  }
}
