import { defineView } from "@lark.js/mvc";
import template from "./dashboard.html";
import useDashboardStore from "@/store/dashboard";
import type { EntrySnapshot } from "@/store/dashboard";

const ROW_HEIGHT = 36;
const OVERSCAN = 5;
const WS_URL = "ws://localhost:9090/dashboard/ws";

interface FlatRow {
  group: string;
  key: string;
  size: number;
  sizeStr: string;
  level: number;
  expire_at: number;
  expireStr: string;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + " B";
  return (bytes / 1024).toFixed(1) + " KB";
}

function formatExpire(nanos: number): string {
  if (nanos <= 0 || nanos >= Number.MAX_SAFE_INTEGER) return "never";
  const ms = nanos / 1_000_000;
  const now = Date.now();
  const diff = ms - now;
  if (diff <= 0) return "expired";
  if (diff < 60_000) return Math.ceil(diff / 1000) + "s left";
  if (diff < 3_600_000) return Math.ceil(diff / 60_000) + "m left";
  return new Date(ms).toLocaleTimeString();
}

function flatten(groups: { name: string; entries: EntrySnapshot[] }[]): FlatRow[] {
  const rows: FlatRow[] = [];
  for (const g of groups) {
    if (!g.entries) continue;
    for (const e of g.entries) {
      rows.push({
        group: g.name,
        key: e.key,
        size: e.size,
        sizeStr: formatSize(e.size),
        level: e.level,
        expire_at: e.expire_at,
        expireStr: formatExpire(e.expire_at),
      });
    }
  }
  return rows;
}

export default defineView({
  template,
  allRows: [] as FlatRow[],
  scrollTop: 0,
  pollTimer: 0,

  init() {
    const store = useDashboardStore();
    store.connect(WS_URL);

    this.pollTimer = window.setInterval(() => {
      const s = useDashboardStore();
      this.allRows = flatten(s.groups);
      this.syncView(s.status);
    }, 500);

    this.on("destroy", () => {
      clearInterval(this.pollTimer);
      useDashboardStore().disconnect();
    });

    this.syncView(store.status);
  },

  syncView(status: string) {
    const containerHeight = 700 - 56 - 40;
    const totalRows = this.allRows.length;
    const visibleCount = Math.ceil(containerHeight / ROW_HEIGHT) + OVERSCAN * 2;
    const startIdx = Math.max(0, Math.floor(this.scrollTop / ROW_HEIGHT) - OVERSCAN);
    const endIdx = Math.min(totalRows, startIdx + visibleCount);

    this.updater
      .set({
        status,
        totalEntries: totalRows,
        visibleRows: this.allRows.slice(startIdx, endIdx),
        topPad: startIdx * ROW_HEIGHT,
        bottomPad: Math.max(0, (totalRows - endIdx) * ROW_HEIGHT),
      })
      .digest();
  },

  "onScroll<scroll>"() {
    const el = document.getElementById("vs-container");
    if (!el) return;
    this.scrollTop = el.scrollTop;
    this.syncView(useDashboardStore().status);
  },

  "deleteEntry<click>"(e: Record<string, unknown>) {
    const p = e.params as Record<string, string>;
    useDashboardStore().deleteKey(p.group, p.key);
  },

  "reconnect<click>"() {
    useDashboardStore().connect(WS_URL);
  },
});
