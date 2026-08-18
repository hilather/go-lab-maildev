import { useEffect, useState } from "react";

export const POLL_INTERVAL_MS = 3000;

export type LiveMode = "idle" | "sse" | "poll";

export function useInboxLive(onChange: () => void, enabled: boolean): LiveMode {
  const [mode, setMode] = useState<LiveMode>("idle");

  useEffect(() => {
    if (!enabled) {
      setMode("idle");
      return;
    }
    let es: EventSource | null = null;
    let poll: number | undefined;
    let opened = false;

    const refresh = () => {
      onChange();
    };

    const startPoll = () => {
      if (poll !== undefined) {
        return;
      }
      setMode("poll");
      poll = window.setInterval(refresh, POLL_INTERVAL_MS);
    };

    try {
      es = new EventSource("/v1/events/stream");
      es.addEventListener("mail.received", refresh);
      es.addEventListener("mail.deleted", refresh);
      es.addEventListener("store.wiped", refresh);
      es.onopen = () => {
        opened = true;
        setMode("sse");
      };
      es.onerror = () => {
        startPoll();
      };
    } catch {
      startPoll();
    }

    const watchdog = window.setTimeout(() => {
      if (!opened) {
        startPoll();
      }
    }, POLL_INTERVAL_MS);

    return () => {
      window.clearTimeout(watchdog);
      if (poll !== undefined) {
        window.clearInterval(poll);
      }
      if (es) {
        es.close();
      }
    };
  }, [enabled, onChange]);

  return mode;
}
