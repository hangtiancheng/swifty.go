import { useCallback, useEffect, useRef, useState } from "react";
import { ChartBar } from "lucide-react";
import useDashboardStore, { type GroupSnapshot } from "../store/dashboard";
import { WS_URL } from "../config";
import { formatSize, formatExpire } from "../utils/format";

const ROW_HEIGHT = 36;
const OVER_SCAN = 5;
const DASHBOARD_WS = WS_URL + "/dashboard/ws";

interface FlatRow {
  group: string;
  key: string;
  size: number;
  sizeStr: string;
  level: number;
  expire_at: number;
  expireStr: string;
}

function flatten(groups: GroupSnapshot[]): FlatRow[] {
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

export default function Dashboard() {
  const [totalEntries, setTotalEntries] = useState(0);
  const [status, setStatus] = useState("disconnected");
  const [visibleRows, setVisibleRows] = useState<FlatRow[]>([]);
  const [topPad, setTopPad] = useState(0);
  const [bottomPad, setBottomPad] = useState(0);

  const containerRef = useRef<HTMLDivElement>(null);
  const scrollTopRef = useRef(0);
  const rafRef = useRef(0);
  const allRowsRef = useRef<FlatRow[]>([]);

  const syncView = useCallback((s: string) => {
    const el = containerRef.current;
    if (!el) return;
    const rows = allRowsRef.current;
    const containerHeight = el.clientHeight;
    const totalRows = rows.length;
    const visibleCount =
      Math.ceil(containerHeight / ROW_HEIGHT) + OVER_SCAN * 2;
    const startIdx = Math.max(
      0,
      Math.floor(scrollTopRef.current / ROW_HEIGHT) - OVER_SCAN,
    );
    const endIdx = Math.min(totalRows, startIdx + visibleCount);
    setTotalEntries(totalRows);
    setVisibleRows(rows.slice(startIdx, endIdx));
    setTopPad(startIdx * ROW_HEIGHT);
    setBottomPad(Math.max(0, (totalRows - endIdx) * ROW_HEIGHT));
    setStatus(s);
  }, []);

  useEffect(() => {
    useDashboardStore.getState().connect(DASHBOARD_WS);
    const pollTimer = window.setInterval(() => {
      const s = useDashboardStore.getState();
      allRowsRef.current = flatten(s.groups);
      syncView(s.status);
    }, 500);
    syncView(useDashboardStore.getState().status);
    return () => {
      clearInterval(pollTimer);
      cancelAnimationFrame(rafRef.current);
      useDashboardStore.getState().disconnect();
    };
  }, [syncView]);

  const onScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    scrollTopRef.current = el.scrollTop;
    cancelAnimationFrame(rafRef.current);
    rafRef.current = requestAnimationFrame(() => {
      syncView(useDashboardStore.getState().status);
    });
  };

  const reconnect = () => {
    useDashboardStore.getState().connect(DASHBOARD_WS);
  };

  const deleteEntry = (group: string, key: string) => {
    useDashboardStore.getState().deleteKey(group, key);
  };

  return (
    <div className="bg-base-200 flex min-h-screen items-center justify-center p-4">
      <div className="card card-border border-base-300 bg-base-100 flex h-175 w-250 flex-col overflow-hidden shadow-xl">
        <div className="border-base-300 bg-base-200 flex h-14 items-center justify-between border-b px-6">
          <div className="flex items-center gap-3">
            <span className="text-primary h-5 w-5">
              <ChartBar size={20} />
            </span>
            <span className="text-base-content text-lg font-semibold">
              Cache Dashboard
            </span>
          </div>
          <div className="flex items-center gap-3">
            {status === "connected" && (
              <span className="badge badge-sm badge-success">Connected</span>
            )}
            {status === "connecting" && (
              <span className="badge badge-sm badge-warning">Connecting</span>
            )}
            {status !== "connected" && status !== "connecting" && (
              <span className="badge badge-sm badge-error">Disconnected</span>
            )}
            <span className="text-base-content/60 text-xs">
              {totalEntries} entries
            </span>
          </div>
        </div>

        {status !== "connected" && (
          <div className="flex flex-1 items-center justify-center">
            <div className="text-center">
              <p className="text-base-content/40 mb-4">
                WebSocket not connected
              </p>
              <button
                className="btn btn-sm btn-accent font-normal"
                onClick={reconnect}
              >
                Connect
              </button>
            </div>
          </div>
        )}

        {status === "connected" && (
          <div className="flex flex-1 flex-col overflow-hidden">
            <div className="border-base-300 bg-base-200/50 text-base-content flex h-10 shrink-0 items-center gap-2 border-b px-4 text-xs font-medium">
              <div className="w-16 text-center">Group</div>
              <div className="flex-1">Key</div>
              <div className="w-16 text-right">Size</div>
              <div className="w-14 text-center">Level</div>
              <div className="w-40 text-center">Expires</div>
              <div className="w-16 text-center">Action</div>
            </div>

            <div
              ref={containerRef}
              className="flex-1 overflow-y-auto"
              onScroll={onScroll}
            >
              <div
                style={{
                  paddingTop: `${topPad}px`,
                  paddingBottom: `${bottomPad}px`,
                }}
              >
                {visibleRows.map((row) => (
                  <div
                    key={row.group + row.key}
                    className="border-base-200 hover:bg-base-200/50 flex h-9 items-center gap-2 border-b px-4 text-sm"
                  >
                    <div className="w-16 text-center">
                      <span className="badge badge-xs badge-outline text-xs">
                        {row.group}
                      </span>
                    </div>
                    <div
                      className="flex-1 truncate font-mono text-xs"
                      title={row.key}
                    >
                      {row.key}
                    </div>
                    <div className="text-base-content/60 w-16 text-right text-xs">
                      {row.sizeStr}
                    </div>
                    <div className="w-14 text-center">
                      {row.level === 1 ? (
                        <span className="badge badge-xs badge-success">
                          hot
                        </span>
                      ) : (
                        <span className="badge badge-xs badge-ghost">cold</span>
                      )}
                    </div>
                    <div className="text-base-content/60 w-40 text-center text-xs">
                      {row.expireStr}
                    </div>
                    <div className="w-16 text-center">
                      <button
                        className="btn btn-xs btn-error btn-outline font-normal"
                        onClick={() => deleteEntry(row.group, row.key)}
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
