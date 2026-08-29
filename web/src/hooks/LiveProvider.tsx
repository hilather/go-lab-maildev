import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { APIError, getStatus } from "../api/client";
import { useInboxLive, type LiveMode } from "./useInboxLive";

type LiveContextValue = {
  mode: LiveMode;
  unreadCount: number;
  decrementUnread: () => void;
  subscribeRefresh: (fn: () => void) => () => void;
};

const LiveContext = createContext<LiveContextValue | null>(null);

export function LiveProvider({ children, enabled = true }: { children: ReactNode; enabled?: boolean }) {
  const listeners = useRef(new Set<() => void>());
  const [unreadCount, setUnreadCount] = useState(0);

  const refreshStatus = useCallback(async () => {
    try {
      const st = await getStatus();
      setUnreadCount(st.store.unreadCount);
    } catch (err) {
      if (err instanceof APIError && err.problem.status === 401) {
        return;
      }
    }
  }, []);

  const onChange = useCallback(() => {
    listeners.current.forEach((fn) => {
      fn();
    });
    void refreshStatus();
  }, [refreshStatus]);

  const mode = useInboxLive(onChange, enabled);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    void refreshStatus();
  }, [enabled, refreshStatus]);

  const subscribeRefresh = useCallback((fn: () => void) => {
    listeners.current.add(fn);
    return () => {
      listeners.current.delete(fn);
    };
  }, []);

  const decrementUnread = useCallback(() => {
    setUnreadCount((n) => Math.max(0, n - 1));
  }, []);

  const value = useMemo(
    () => ({ mode, unreadCount, decrementUnread, subscribeRefresh }),
    [mode, unreadCount, decrementUnread, subscribeRefresh],
  );

  return <LiveContext.Provider value={value}>{children}</LiveContext.Provider>;
}

export function useLive(): LiveContextValue {
  const ctx = useContext(LiveContext);
  if (!ctx) {
    throw new Error("useLive requires LiveProvider");
  }
  return ctx;
}
