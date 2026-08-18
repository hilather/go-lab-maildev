import { useEffect, useState } from "react";

export const POLL_INTERVAL_MS = 3000;
export const SSE_WATCHDOG_MS = 15000;

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
    let exclusivePoll = false;

    const refresh = () => {
      onChange();
    };

    const startPoll = (interval: number) => {
      if (poll !== undefined) {
        window.clearInterval(poll);
      }
      poll = window.setInterval(refresh, interval);
    };

    // Exclusive fallback: close EventSource so the browser cannot keep
    // reconnecting while poll is the live path.
    const fallbackToPoll = () => {
      exclusivePoll = true;
      if (es !== null) {
        es.close();
        es = null;
      }
      setMode("poll");
      startPoll(POLL_INTERVAL_MS);
    };

    try {
      es = new EventSource("/v1/events/stream");
      es.addEventListener("mail.received", refresh);
      es.addEventListener("mail.deleted", refresh);
      es.addEventListener("store.wiped", refresh);
      es.onopen = () => {
        // close() can still deliver a queued open after fallbackToPoll.
        if (exclusivePoll || es === null) {
          return;
        }
        opened = true;
        setMode("sse");
        // Recover silently dropped SSE events (store fan-out is drop-oldest).
        startPoll(SSE_WATCHDOG_MS);
      };
      es.onerror = () => {
        fallbackToPoll();
      };
    } catch {
      fallbackToPoll();
    }

    const watchdog = window.setTimeout(() => {
      if (!opened) {
        fallbackToPoll();
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
