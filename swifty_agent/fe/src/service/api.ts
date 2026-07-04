const API_BASE = "http://localhost:6872/api";

export interface ChatRequest {
  Id: string;
  Question: string;
}

export interface ChatResponse {
  message: string;
  data?: { answer?: string };
}

export interface AIOpsResponse {
  message: string;
  data?: { result?: string; detail?: string[] };
}

export async function sendChatQuick(
  sessionId: string,
  question: string,
): Promise<ChatResponse> {
  const res = await fetch(`${API_BASE}/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ Id: sessionId, Question: question }),
  });
  return res.json();
}

export async function uploadFile(file: File): Promise<unknown> {
  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch(`${API_BASE}/upload`, {
    method: "POST",
    body: formData,
  });
  return res.json();
}

export async function triggerAIOps(): Promise<AIOpsResponse> {
  const res = await fetch(`${API_BASE}/ai_ops`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  return res.json();
}

export function sendChatStream(
  sessionId: string,
  question: string,
  onMessage: (content: string) => void,
  onDone: () => void,
  onError: (err: string) => void,
): () => void {
  const controller = new AbortController();

  fetch(`${API_BASE}/chat_stream`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ Id: sessionId, Question: question }),
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        onError(`HTTP error: ${response.status}`);
        return;
      }

      const reader = response.body?.getReader();
      if (!reader) {
        onError("No response body");
        return;
      }

      const decoder = new TextDecoder();
      let buffer = "";
      let currentEvent = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (line.trim() === "") continue;

          if (line.startsWith("event: ")) {
            currentEvent = line.substring(7).trim();
            if (currentEvent === "done") {
              onDone();
              return;
            }
          } else if (line.startsWith("data: ")) {
            const data = line.substring(6);
            if (data === "[DONE]") {
              onDone();
              return;
            }
            if (currentEvent === "message") {
              onMessage(data === "" ? "\n" : data);
            } else if (currentEvent === "error") {
              onError(data);
            }
          }
        }
      }
      onDone();
    })
    .catch((err) => {
      if (err.name !== "AbortError") {
        onError(err.message);
      }
    });

  return () => controller.abort();
}
