import { defineStore } from "@lark.js/mvc";
import { WS_URL } from "@/config";

export interface WsState {
  status: string;
  onMessageHandler: ((msg: MessageEvent) => void) | null;
  connect: (uuid: string) => void;
  disconnect: () => void;
  send: (data: unknown) => void;
  setOnMessage: (handler: (msg: MessageEvent) => void) => void;
}

let rawSocket: WebSocket | null = null;

const useWsStore = defineStore("ws", (s) => {
  const store = s as unknown as WsState;
  return {
    status: "disconnected",
    onMessageHandler: null as ((msg: MessageEvent) => void) | null,

    connect(uuid: string) {
      if (rawSocket) rawSocket.close();
      store.status = "connecting";
      const ws = new WebSocket(WS_URL + "/wss?client_id=" + uuid);
      ws.onopen = () => {
        store.status = "connected";
      };
      ws.onmessage = (msg: MessageEvent) => {
        if (store.onMessageHandler) store.onMessageHandler(msg);
      };
      ws.onclose = () => {
        store.status = "disconnected";
        rawSocket = null;
      };
      ws.onerror = () => {
        store.status = "disconnected";
      };
      rawSocket = ws;
    },

    disconnect() {
      if (rawSocket) rawSocket.close();
      rawSocket = null;
      store.status = "disconnected";
    },

    send(data: unknown) {
      if (rawSocket && rawSocket.readyState === WebSocket.OPEN) {
        rawSocket.send(JSON.stringify(data));
      }
    },

    setOnMessage(handler: (msg: MessageEvent) => void) {
      store.onMessageHandler = handler;
    },
  };
}) as unknown as {
  (): WsState;
  (view: unknown): WsState;
};

export default useWsStore;
