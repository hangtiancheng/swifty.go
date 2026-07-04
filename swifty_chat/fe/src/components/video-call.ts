import { LitElement, html, css } from "lit";
import { customElement, state } from "lit/decorators.js";
import { RtcManager } from "../utils/rtc";
import { showToast } from "../utils/toast";

@customElement("video-call")
export class VideoCall extends LitElement {
  protected override createRenderRoot(): HTMLElement {
    return this;
  }

  static override styles = css`
    :host {
      display: block;
    }
  `;

  @state() private visible = false;
  @state() private inCall = false;
  @state() private incomingCall = false;

  private rtc = new RtcManager();

  override connectedCallback(): void {
    super.connectedCallback();
    this.rtc.onLocalStream = (stream) => {
      const el = this.querySelector("#local-video") as HTMLVideoElement | null;
      if (el) el.srcObject = stream;
    };
    this.rtc.onRemoteStream = (stream) => {
      const el = this.querySelector("#remote-video") as HTMLVideoElement | null;
      if (el) el.srcObject = stream;
    };
    this.rtc.onCallEnded = () => {
      this.inCall = false;
      this.incomingCall = false;
      showToast("Call ended", "info");
    };
  }

  override disconnectedCallback(): void {
    this.rtc.endCall();
    super.disconnectedCallback();
  }

  /** Public: open the video call modal. Called from React via ref. */
  show() {
    this.visible = true;
  }

  /** Public: handle an incoming WebRTC signal delivered via websocket. */
  handleSignal(avData: Record<string, unknown>) {
    const result = this.rtc.handleSignal(avData);
    if (result === "incoming_call") {
      this.visible = true;
      this.incomingCall = true;
      showToast("Incoming call", "info");
    }
  }

  private async startCall() {
    await this.rtc.startCall();
    this.inCall = true;
  }

  private async acceptCall() {
    await this.rtc.acceptCall();
    this.inCall = true;
    this.incomingCall = false;
  }

  private rejectCall() {
    this.rtc.rejectCall();
    this.incomingCall = false;
  }

  private hangUp() {
    this.rtc.sendEndCall();
    this.inCall = false;
  }

  private closeModal() {
    if (this.inCall) {
      showToast("Please hang up first", "warning");
      return;
    }
    this.visible = false;
    this.incomingCall = false;
  }

  override render() {
    if (!this.visible) return html``;
    return html`
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      >
        <div
          class="card card-border border-base-300 bg-base-100 w-175 p-6 shadow-2xl"
        >
          <h2 class="text-base-content mb-4 text-center text-lg font-semibold">
            Video Call
          </h2>
          <div class="mb-4 flex justify-center gap-4">
            <div
              class="rounded-box bg-neutral relative h-56.25 w-75 overflow-hidden"
            >
              <video
                id="local-video"
                autoplay
                playsinline
                muted
                class="h-full w-full object-cover"
              ></video>
              <span
                class="badge badge-sm badge-ghost absolute bottom-2 left-2 text-xs"
                >You</span
              >
            </div>
            <div
              class="rounded-box bg-neutral relative h-56.25 w-75 overflow-hidden"
            >
              <video
                id="remote-video"
                autoplay
                playsinline
                class="h-full w-full object-cover"
              ></video>
              <span
                class="badge badge-sm badge-ghost absolute bottom-2 left-2 text-xs"
                >Remote</span
              >
            </div>
          </div>

          ${
            this.incomingCall
              ? html`<p class="text-primary mb-3 text-center text-sm">
                  Incoming call...
                </p>`
              : null
          }

          <div class="flex justify-center gap-3">
            ${
              !this.inCall && !this.incomingCall
                ? html`<button
                    class="btn btn-sm btn-accent font-normal"
                    @click=${this.startCall}
                  >
                    Start Call
                  </button>`
                : null
            }
            ${
              this.incomingCall
                ? html`<button
                      class="btn btn-sm btn-accent font-normal"
                      @click=${this.acceptCall}
                    >
                      Accept
                    </button>
                    <button
                      class="btn btn-sm btn-error font-normal"
                      @click=${this.rejectCall}
                    >
                      Reject
                    </button>`
                : null
            }
            ${
              this.inCall
                ? html`<button
                    class="btn btn-sm btn-error font-normal"
                    @click=${this.hangUp}
                  >
                    Hang Up
                  </button>`
                : null
            }
            <button
              class="btn btn-sm btn-ghost text-base-content/60 font-normal"
              @click=${this.closeModal}
            >
              Close
            </button>
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "video-call": VideoCall;
  }
}
