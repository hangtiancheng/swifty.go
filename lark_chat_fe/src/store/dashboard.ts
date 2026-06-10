import { defineStore } from "@lark.js/mvc";

export interface EntrySnapshot {
  key: string;
  size: number;
  expire_at: number;
  level: number;
}

export interface GroupSnapshot {
  name: string;
  stats: Record<string, unknown>;
  entries: EntrySnapshot[];
}

export interface DashboardState {
  groups: GroupSnapshot[];
  status: string;
  ws: WebSocket | null;
  connect: (url: string) => void;
  disconnect: () => void;
  deleteKey: (group: string, key: string) => void;
}

const useDashboardStore = defineStore("dashboard", (s) => {
  const store = s as unknown as DashboardState & {
    _retryTimer: number;
    _intentionalClose: boolean;
  };
  return {
    groups: [] as GroupSnapshot[],
    status: "disconnected",
    ws: null as WebSocket | null,
    _retryTimer: 0,
    _intentionalClose: false,

    connect(url: string) {
      store._intentionalClose = false;
      if (store.ws) store.ws.close();
      if (store._retryTimer) clearTimeout(store._retryTimer);
      store.status = "connecting";

      const ws = new WebSocket(url);
      ws.onopen = () => {
        store.status = "connected";
      };
      ws.onmessage = (e: MessageEvent) => {
        const data = JSON.parse(e.data);
        if (data.type === "snapshot") {
          store.groups = data.groups ?? [];
        }
      };
      ws.onclose = () => {
        store.status = "disconnected";
        store.ws = null;
        if (!store._intentionalClose) {
          store._retryTimer = window.setTimeout(() => store.connect(url), 3000);
        }
      };
      ws.onerror = () => {
        store.status = "disconnected";
      };
      store.ws = ws;
    },

    disconnect() {
      store._intentionalClose = true;
      if (store._retryTimer) clearTimeout(store._retryTimer);
      if (store.ws) store.ws.close();
      store.ws = null;
      store.status = "disconnected";
      store.groups = [];
    },

    deleteKey(group: string, key: string) {
      if (store.ws && store.ws.readyState === WebSocket.OPEN) {
        store.ws.send(JSON.stringify({ action: "delete", group, key }));
      }
    },
  };
}) as unknown as {
  (): DashboardState;
  (view: unknown): DashboardState;
};

export default useDashboardStore;
