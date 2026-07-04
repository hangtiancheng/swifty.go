import { create } from "zustand";
import type { ContactInfo, Message } from "../types";

export interface ChatState {
  contactInfo: ContactInfo | null;
  sessionId: string;
  messageList: Message[];
  setContact: (info: ContactInfo) => void;
  setSessionId: (id: string) => void;
  addMessage: (msg: Message) => void;
  setMessageList: (list: Message[]) => void;
  clearChat: () => void;
}

const useChatStore = create<ChatState>((set, get) => ({
  contactInfo: null as ContactInfo | null,
  sessionId: "",
  messageList: [] as Message[],

  setContact(info: ContactInfo) {
    set({ contactInfo: info });
  },
  setSessionId(id: string) {
    set({ sessionId: id });
  },
  addMessage(msg: Message) {
    set({ messageList: [...get().messageList, msg] });
  },
  setMessageList(list: Message[]) {
    set({ messageList: list || [] });
  },
  clearChat() {
    set({ contactInfo: null, sessionId: "", messageList: [] });
  },
}));

export default useChatStore;
