import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { POLL_INTERVAL_MS, useInboxLive } from "./useInboxLive";

class FakeEventSource {
  static fail = false;
  static instances: FakeEventSource[] = [];
  onopen: ((ev: Event) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  readonly listeners = new Map<string, Set<(ev: MessageEvent<string>) => void>>();

  constructor(public readonly url: string) {
    FakeEventSource.instances.push(this);
    if (FakeEventSource.fail) {
      throw new Error("EventSource unavailable");
    }
  }

  addEventListener(type: string, fn: (ev: MessageEvent<string>) => void): void {
    const set = this.listeners.get(type) ?? new Set();
    set.add(fn);
    this.listeners.set(type, set);
  }

  removeEventListener(): void {}
  close(): void {}
}

describe("useInboxLive", () => {
  afterEach(() => {
    FakeEventSource.fail = false;
    FakeEventSource.instances = [];
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("uses EventSource when it opens", async () => {
    vi.stubGlobal("EventSource", FakeEventSource);
    const onChange = vi.fn();
    const { result } = renderHook(() => useInboxLive(onChange, true));
    FakeEventSource.instances[0]?.onopen?.(new Event("open"));
    await waitFor(() => {
      expect(result.current).toBe("sse");
    });
    expect(FakeEventSource.instances[0]?.url).toBe("/v1/events/stream");
  });

  it("falls back to a 3s poll when EventSource cannot be constructed", async () => {
    FakeEventSource.fail = true;
    vi.stubGlobal("EventSource", FakeEventSource);
    const onChange = vi.fn();
    const { result } = renderHook(() => useInboxLive(onChange, true));
    await waitFor(() => {
      expect(result.current).toBe("poll");
    });
    expect(POLL_INTERVAL_MS).toBe(3000);
  });
});
