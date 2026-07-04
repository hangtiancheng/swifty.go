export interface ChatMessage {
  id: string;
  type: "user" | "assistant" | "loading";
  content: string;
  timestamp: string;
  /** Optional AI Ops step breakdown rendered as a collapsible list. */
  details?: string[];
}

export interface ChatHistoryItem {
  id: string;
  title: string;
  messages: ChatMessage[];
  createdAt: string;
  updatedAt: string;
}
