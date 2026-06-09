import { defineStore } from "@lark.js/mvc";
import { WS_URL } from "@/config";

export interface WsState {
  socket: WebSocket | null;
  status: string;
  onMessageHandler: ((msg: MessageEvent) => void) | null;
  connect: (uuid: string) => void;
  disconnect: () => void;
  send: (data: unknown) => void;
  setOnMessage: (handler: (msg: MessageEvent) => void) => void;
}

const useWsStore = defineStore("ws", (s) => {
  const store = s as unknown as WsState;
  return {
    socket: null as WebSocket | null,
    status: "disconnected",
    onMessageHandler: null as ((msg: MessageEvent) => void) | null,

    connect(uuid: string) {
      if (store.socket) store.socket.close();
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
        store.socket = null;
      };
      ws.onerror = () => {
        store.status = "disconnected";
      };
      store.socket = ws;
    },

    disconnect() {
      if (store.socket) store.socket.close();
      store.socket = null;
      store.status = "disconnected";
    },

    send(data: unknown) {
      if (store.socket && store.socket.readyState === WebSocket.OPEN) {
        store.socket.send(JSON.stringify(data));
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
