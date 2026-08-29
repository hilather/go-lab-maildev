import { screen } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "../App";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { containsForbiddenControl } from "./forbidden";

class FakeEventSource {
  addEventListener(): void {}
  close(): void {}
}

describe("AppShell", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders dark-shell chips and scoped rail without send controls", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/status")) {
          return json(200, {
            ready: true,
            revisions: {},
            listeners: [],
            store: { messageCount: 0, storeBytes: 0, unreadCount: 2, storeGeneration: 1, epoch: 1 },
          });
        }
        return json(200, { revision: "r", storeGeneration: 1, items: [], nextCursor: null });
      }),
    );
    vi.stubGlobal("EventSource", FakeEventSource);

    renderApp(
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<p>inbox-body</p>} />
        </Route>
      </Routes>,
    );

    expect(await screen.findByText("receive-only")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /sign out/i })).toBeInTheDocument();
    const labels = ["Inbox", "Status", "Audit", "Reset"];
    for (const label of labels) {
      expect(screen.getByRole("link", { name: new RegExp(label, "i") })).toBeInTheDocument();
    }
    expect(containsForbiddenControl(labels)).toBe(false);
    expect(await screen.findByText("2")).toBeInTheDocument();
  });
});
