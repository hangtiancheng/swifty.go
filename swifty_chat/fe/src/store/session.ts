import { create } from "zustand";
import type { SessionItem } from "../types";

export interface SessionState {
  userSessions: SessionItem[];
  groupSessions: SessionItem[];
  setUserSessions: (list: SessionItem[]) => void;
  setGroupSessions: (list: SessionItem[]) => void;
  clear: () => void;
}

const useSessionStore = create<SessionState>((set) => ({
  userSessions: [] as SessionItem[],
  groupSessions: [] as SessionItem[],

  setUserSessions(list: SessionItem[]) {
    set({ userSessions: list || [] });
  },
  setGroupSessions(list: SessionItem[]) {
    set({ groupSessions: list || [] });
  },
  clear() {
    set({ userSessions: [], groupSessions: [] });
  },
}));

export default useSessionStore;
