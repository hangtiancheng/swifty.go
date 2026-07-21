/**
 * Copyright (c) 2026 hangtiancheng
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

import { create } from "zustand";
import { WS_URL } from "../config";

export interface WsState {
  status: string;
  onMessageHandler: ((msg: MessageEvent) => void) | null;
  connect: (uuid: string) => void;
  disconnect: () => void;
  send: (data: unknown) => void;
  setOnMessage: (handler: (msg: MessageEvent) => void) => void;
}

let rawSocket: WebSocket | null = null;

const useWsStore = create<WsState>((set, get) => ({
  status: "disconnected",
  onMessageHandler: null as ((msg: MessageEvent) => void) | null,

  connect(uuid: string) {
    if (rawSocket) rawSocket.close();
    set({ status: "connecting" });
    const ws = new WebSocket(WS_URL + "/wss?client_id=" + uuid);
    ws.onopen = () => {
      set({ status: "connected" });
    };
    ws.onmessage = (msg: MessageEvent) => {
      const handler = get().onMessageHandler;
      if (handler) handler(msg);
    };
    ws.onclose = () => {
      set({ status: "disconnected" });
      rawSocket = null;
    };
    ws.onerror = () => {
      set({ status: "disconnected" });
    };
    rawSocket = ws;
  },

  disconnect() {
    if (rawSocket) rawSocket.close();
    rawSocket = null;
    set({ status: "disconnected" });
  },

  send(data: unknown) {
    if (rawSocket && rawSocket.readyState === WebSocket.OPEN) {
      rawSocket.send(JSON.stringify(data));
    }
  },

  setOnMessage(handler: (msg: MessageEvent) => void) {
    set({ onMessageHandler: handler });
  },
}));

export default useWsStore;
