import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { marked } from "marked";
import hljs from "highlight.js";

marked.setOptions({ breaks: true, gfm: true });

const renderer = new marked.Renderer();
renderer.code = function ({ text, lang }: { text: string; lang?: string }) {
  const language = lang && hljs.getLanguage(lang) ? lang : "plaintext";
  const highlighted = hljs.highlight(text, { language }).value;
  return `<pre><code class="hljs language-${language}">${highlighted}</code></pre>`;
};

function renderMarkdown(content: string): string {
  if (!content) return "";
  try {
    return marked.parse(content, { renderer }) as string;
  } catch {
    const el = document.createElement("div");
    el.textContent = content;
    return el.innerHTML;
  }
}

@customElement("markdown-content")
export class MarkdownContent extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @property({ type: String })
  content = "";

  override render() {
    const htmlContent = renderMarkdown(this.content);
    return html`<div class="prose prose-sm max-w-none">
      ${unsafeHTML(htmlContent)}
    </div>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "markdown-content": MarkdownContent;
  }
}
