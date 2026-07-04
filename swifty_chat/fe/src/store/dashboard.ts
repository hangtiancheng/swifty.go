import { create } from "zustand";

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

const useDashboardStore = create<DashboardState>((set, get) => ({
  groups: [] as GroupSnapshot[],
  status: "disconnected",
  ws: null as WebSocket | null,
  _retryTimer: 0,
  _intentionalClose: false,

  connect(url: string) {
    const state = get() as DashboardState & {
      _retryTimer: number;
      _intentionalClose: boolean;
    };
    state._intentionalClose = false;
    if (state.ws) state.ws.close();
    if (state._retryTimer) clearTimeout(state._retryTimer);
    set({ status: "connecting" });

    const ws = new WebSocket(url);
    ws.onopen = () => {
      set({ status: "connected" });
    };
    ws.onmessage = (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      if (data.type === "snapshot") {
        set({ groups: data.groups ?? [] });
      }
    };
    ws.onclose = () => {
      const s = get() as DashboardState & {
        _retryTimer: number;
        _intentionalClose: boolean;
      };
      set({ status: "disconnected", ws: null });
      if (!s._intentionalClose) {
        s._retryTimer = window.setTimeout(() => s.connect(url), 3000);
      }
    };
    ws.onerror = () => {
      set({ status: "disconnected" });
    };
    set({ ws } as unknown as Partial<DashboardState>);
  },

  disconnect() {
    const state = get() as DashboardState & {
      _retryTimer: number;
      _intentionalClose: boolean;
    };
    state._intentionalClose = true;
    if (state._retryTimer) clearTimeout(state._retryTimer);
    if (state.ws) state.ws.close();
    set({
      ws: null,
      status: "disconnected",
      groups: [],
    });
  },

  deleteKey(group: string, key: string) {
    const state = get() as DashboardState & {
      _retryTimer: number;
      _intentionalClose: boolean;
    };
    if (state.ws && state.ws.readyState === WebSocket.OPEN) {
      state.ws.send(JSON.stringify({ action: "delete", group, key }));
    }
  },
}));

export default useDashboardStore;
