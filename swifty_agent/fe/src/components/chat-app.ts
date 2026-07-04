import { LitElement, html, css } from "lit";
import { customElement } from "lit/decorators.js";
import useChatStore from "../store/chat";
import type { ChatMessage, ChatHistoryItem } from "../types";
import {
  sendChatQuick,
  sendChatStream,
  uploadFile as uploadFileApi,
  triggerAIOps as triggerAIOpsApi,
} from "../service/api";
import "./chat-sidebar";
import "./chat-topbar";
import "./chat-welcome";
import "./chat-messages";
import "./chat-input";
import "./chat-loading";

let msgIdCounter = 0;
function nextMsgId(): string {
  return `msg_${++msgIdCounter}_${Date.now()}`;
}

@customElement("chat-app")
export class ChatApp extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
      height: 100vh;
      overflow: hidden;
    }
  `;

  private _unsubscribe?: () => void;

  private _messages: ChatMessage[] = [];
  private _isStreaming = false;
  private _currentMode: "quick" | "stream" = "quick";
  private _chatHistories: ChatHistoryItem[] = [];
  private _isCentered = true;
  private _showLoading = false;
  private _sessionId = "";

  override connectedCallback() {
    super.connectedCallback();
    this._unsubscribe = useChatStore.subscribe((state) => {
      this._messages = state.messages;
      this._isStreaming = state.isStreaming;
      this._currentMode = state.currentMode;
      this._chatHistories = state.chatHistories;
      this._isCentered = state.isCentered;
      this._showLoading = state.showLoading;
      this._sessionId = state.sessionId;
      this.requestUpdate();
    });
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    this._unsubscribe?.();
  }

  override render() {
    return html`
      <div class="bg-base-100 flex h-screen overflow-hidden">
        <chat-sidebar
          .chatHistories=${this._chatHistories}
          .sessionId=${this._sessionId}
          @new-chat=${this._handleNewChat}
          @load-history=${this._handleLoadHistory}
          @delete-history=${this._handleDeleteHistory}
        ></chat-sidebar>

        <main class="bg-base-100 relative flex min-w-0 flex-1 flex-col">
          <chat-topbar @trigger-aiops=${this._handleTriggerAIOps}></chat-topbar>

          <div
            class="${this._isCentered ? "justify-center" : ""} flex min-h-0 flex-1 flex-col"
          >
            ${this._isCentered ? html`<chat-welcome></chat-welcome>` : ""}
            <chat-messages
              .messages=${this._messages}
              .isStreaming=${this._isStreaming}
            ></chat-messages>
            <chat-input
              .isStreaming=${this._isStreaming}
              .currentMode=${this._currentMode}
              @send-message=${this._handleSendMessage}
              @file-selected=${this._handleFileSelected}
              @mode-change=${this._handleModeChange}
            ></chat-input>
          </div>
        </main>
      </div>

      <chat-loading .open=${this._showLoading}></chat-loading>
    `;
  }

  private _handleNewChat() {
    useChatStore.getState().newSession();
  }

  private _handleLoadHistory(e: CustomEvent<{ id: string }>) {
    useChatStore.getState().loadChatHistory(e.detail.id);
  }

  private _handleDeleteHistory(e: CustomEvent<{ id: string }>) {
    useChatStore.getState().deleteChatHistory(e.detail.id);
  }

  private _handleModeChange(e: CustomEvent<{ mode: "quick" | "stream" }>) {
    useChatStore.getState().setMode(e.detail.mode);
  }

  private async _handleSendMessage(e: CustomEvent<{ content: string }>) {
    const message = e.detail.content;
    const state = useChatStore.getState();
    if (state.isStreaming) return;

    const userMsg: ChatMessage = {
      id: nextMsgId(),
      type: "user",
      content: message,
      timestamp: new Date().toISOString(),
    };
    state.addMessage(userMsg);
    state.setStreaming(true);

    try {
      if (state.currentMode === "quick") {
        await this._sendQuickMessage(message);
      } else {
        await this._sendStreamMessage(message);
      }
    } catch (err: unknown) {
      const errMsg = err instanceof Error ? err.message : String(err);
      state.addMessage({
        id: nextMsgId(),
        type: "assistant",
        content: "Error: " + errMsg,
        timestamp: new Date().toISOString(),
      });
    } finally {
      state.setStreaming(false);
    }
  }

  private async _sendQuickMessage(question: string) {
    const state = useChatStore.getState();
    const data = await sendChatQuick(state.sessionId, question);

    if (data?.message === "OK" && data?.data?.answer) {
      state.addMessage({
        id: nextMsgId(),
        type: "assistant",
        content: data.data.answer,
        timestamp: new Date().toISOString(),
      });
    } else {
      throw new Error(data?.message || "Unknown error");
    }
  }

  private async _sendStreamMessage(question: string) {
    const state = useChatStore.getState();
    const assistantMsgId = nextMsgId();
    state.addMessage({
      id: assistantMsgId,
      type: "assistant",
      content: "",
      timestamp: new Date().toISOString(),
    });

    let fullResponse = "";
    return new Promise<void>((resolve) => {
      sendChatStream(
        state.sessionId,
        question,
        (content: string) => {
          fullResponse += content;
          useChatStore.getState().updateLastAssistantMessage(fullResponse);
        },
        () => resolve(),
        (err: string) => {
          useChatStore.getState().updateLastAssistantMessage("Error: " + err);
          resolve();
        },
      );
    });
  }

  private async _handleFileSelected(e: CustomEvent<{ file: File }>) {
    const file = e.detail.file;
    const state = useChatStore.getState();
    state.setStreaming(true);
    state.setShowLoading(true);

    try {
      await uploadFileApi(file);
      state.addMessage({
        id: nextMsgId(),
        type: "assistant",
        content: `${file.name} uploaded to knowledge base successfully.`,
        timestamp: new Date().toISOString(),
      });
      this._showNotification("File uploaded successfully.", "success");
    } catch (err: unknown) {
      const errMsg = err instanceof Error ? err.message : String(err);
      this._showNotification("Upload failed: " + errMsg, "error");
    } finally {
      state.setStreaming(false);
      state.setShowLoading(false);
    }
  }

  private async _handleTriggerAIOps() {
    const state = useChatStore.getState();
    if (state.isStreaming) return;

    state.newSession();
    const loadingMsgId = nextMsgId();
    state.addMessage({
      id: loadingMsgId,
      type: "assistant",
      content: "Analyzing...",
      timestamp: new Date().toISOString(),
    });
    state.setStreaming(true);
    state.setShowLoading(true);

    try {
      const data = await triggerAIOpsApi();

      if (data?.message === "OK" && data?.data?.result) {
        let responseText = data.data.result;
        try {
          const parsed = JSON.parse(responseText) as { response?: string };
          responseText = parsed.response || responseText;
        } catch {
          // Use raw result if not JSON.
        }
        useChatStore.getState().updateLastAssistantMessage(responseText);

        const details = data.data.detail || [];
        if (details.length > 0) {
          useChatStore.getState().updateLastAssistantDetails(details);
        }
      } else {
        useChatStore
          .getState()
          .updateLastAssistantMessage("AI Ops: no result.");
      }
    } catch (err: unknown) {
      const errMsg = err instanceof Error ? err.message : String(err);
      useChatStore
        .getState()
        .updateLastAssistantMessage("AI Ops error: " + errMsg);
    } finally {
      useChatStore.getState().setStreaming(false);
      useChatStore.getState().setShowLoading(false);
    }
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
    "chat-app": ChatApp;
  }
}
