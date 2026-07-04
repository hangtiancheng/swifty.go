import { create } from "zustand";
import { WS_URL } from "../config";

export interface WsState {
  status: string;
  onMessageHandler: ((msg: MessageEvent) => void) | null;
  connect: (uuid: string) => void;
  disconnect: () => void;
  send: (data: unknown) => void;
  setOnMessage: (handler: (msg: MessageEvent) => void) => void;
}

let rawSocket: WebSocket | null = null;

const useWsStore = create<WsState>((set, get) => ({
  status: "disconnected",
  onMessageHandler: null as ((msg: MessageEvent) => void) | null,

  connect(uuid: string) {
    if (rawSocket) rawSocket.close();
    set({ status: "connecting" });
    const ws = new WebSocket(WS_URL + "/wss?client_id=" + uuid);
    ws.onopen = () => {
      set({ status: "connected" });
    };
    ws.onmessage = (msg: MessageEvent) => {
      const handler = get().onMessageHandler;
      if (handler) handler(msg);
    };
    ws.onclose = () => {
      set({ status: "disconnected" });
      rawSocket = null;
    };
    ws.onerror = () => {
      set({ status: "disconnected" });
    };
    rawSocket = ws;
  },

  disconnect() {
    if (rawSocket) rawSocket.close();
    rawSocket = null;
    set({ status: "disconnected" });
  },

  send(data: unknown) {
    if (rawSocket && rawSocket.readyState === WebSocket.OPEN) {
      rawSocket.send(JSON.stringify(data));
    }
  },

  setOnMessage(handler: (msg: MessageEvent) => void) {
    set({ onMessageHandler: handler });
  },
}));

export default useWsStore;
