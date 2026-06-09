import { defineStore } from "@lark.js/mvc";
import type { ContactInfo, Message } from "@/types";

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

const useChatStore = defineStore("chat", (s) => {
  const store = s as unknown as ChatState;
  return {
    contactInfo: null as ContactInfo | null,
    sessionId: "",
    messageList: [] as Message[],

    setContact(info: ContactInfo) {
      store.contactInfo = info;
    },
    setSessionId(id: string) {
      store.sessionId = id;
    },
    addMessage(msg: Message) {
      store.messageList = [...store.messageList, msg];
    },
    setMessageList(list: Message[]) {
      store.messageList = list || [];
    },
    clearChat() {
      store.contactInfo = null;
      store.sessionId = "";
      store.messageList = [];
    },
  };
}) as unknown as {
  (): ChatState;
  (view: unknown): ChatState;
};

export default useChatStore;
