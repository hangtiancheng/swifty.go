import { defineView } from "@lark.js/mvc";
import template from "./video-call.html";
import { RtcManager } from "@/utils/rtc";
import { showToast } from "@/utils/toast";

export default defineView({
  template,
  rtc: null as RtcManager | null,

  init() {
    this.updater.set({ visible: false, inCall: false, incomingCall: false }).digest();
    this.rtc = new RtcManager();
    this.rtc.onLocalStream = (stream) => {
      const el = document.getElementById("local-video") as HTMLVideoElement | null;
      if (el) {
        el.srcObject = stream;
      }
    };
    this.rtc.onRemoteStream = (stream) => {
      const el = document.getElementById("remote-video") as HTMLVideoElement | null;
      if (el) {
        el.srcObject = stream;
      }
    };
    this.rtc.onCallEnded = () => {
      this.updater.set({ inCall: false, incomingCall: false }).digest();
      showToast("Call ended", "info");
    };
  },

  show() {
    this.updater.set({ visible: true }).digest();
  },

  handleSignal(avData: Record<string, unknown>) {
    const result = this.rtc!.handleSignal(avData);
    if (result === "incoming_call") {
      this.updater.set({ visible: true, incomingCall: true }).digest();
      showToast("Incoming call", "info");
    }
  },

  "startCall<click>"() {
    this.rtc!.startCall();
    this.updater.set({ inCall: true }).digest();
  },

  "acceptCall<click>"() {
    this.rtc!.acceptCall();
    this.updater.set({ inCall: true, incomingCall: false }).digest();
  },

  "rejectCall<click>"() {
    this.rtc!.rejectCall();
    this.updater.set({ incomingCall: false }).digest();
  },

  "hangUp<click>"() {
    this.rtc!.sendEndCall();
    this.updater.set({ inCall: false }).digest();
  },

  "closeModal<click>"() {
    if (this.updater.get("inCall")) {
      showToast("Please hang up first", "warning");
      return;
    }
    this.updater.set({ visible: false, incomingCall: false }).digest();
  },
});
