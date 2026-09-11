import { LitElement, html } from "lit";
import { customElement } from "lit/decorators.js";
import { setupSentry } from "./sentry.js";
import "./index.css";
import "./components/chat-app.js";

if (import.meta.env.DEV) {
  import("./crash/index.js");
}

setupSentry();

@customElement("app-router")
export class AppRouter extends LitElement {
  createRenderRoot() {
    return this;
  }

  connectedCallback() {
    super.connectedCallback();
    this.style.display = "contents";
  }

  render() {
    return html`<chat-app></chat-app>${
        import.meta.env.DEV ? html`<random-crash></random-crash>` : ""
      }`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "app-router": AppRouter;
  }
}
