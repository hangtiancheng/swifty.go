import { defineStore } from "@lark.js/mvc";
import type { SessionItem } from "@/types";

export interface SessionState {
  userSessions: SessionItem[];
  groupSessions: SessionItem[];
  setUserSessions: (list: SessionItem[]) => void;
  setGroupSessions: (list: SessionItem[]) => void;
  clear: () => void;
}

const useSessionStore = defineStore("session", (s) => {
  const store = s as unknown as SessionState;
  return {
    userSessions: [] as SessionItem[],
    groupSessions: [] as SessionItem[],

    setUserSessions(list: SessionItem[]) {
      store.userSessions = list || [];
    },
    setGroupSessions(list: SessionItem[]) {
      store.groupSessions = list || [];
    },
    clear() {
      store.userSessions = [];
      store.groupSessions = [];
    },
  };
}) as unknown as {
  (): SessionState;
  (view: unknown): SessionState;
};

export default useSessionStore;
