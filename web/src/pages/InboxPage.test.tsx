import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AppShell } from "../App";
import { CSRF_HEADER } from "../api/client";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { InboxPage } from "./InboxPage";

const listItem = {
  id: "01JTEST",
  receivedAt: "2026-08-29T11:28:00Z",
  subject: "Your lab sign-in code is 482193",
  from: [{ name: "App", address: "noreply@app.lab.test" }],
  to: [{ name: "", address: "inbox@lab.test" }],
  cc: [],
  bcc: [],
  messageId: "<1@lab>",
  read: false,
  size: 12,
  hasHTML: true,
  envelope: { from: "noreply@app.lab.test", to: ["inbox@lab.test"], helo: "lab", remoteAddress: "127.0.0.1:1", tls: false },
  attachments: [{ id: "att1", filename: "note.txt", contentType: "text/plain", size: 4, checksum: "ab" }],
};

const fullMessage = {
  ...listItem,
  headers: [{ name: "Subject", value: listItem.subject }],
  text: "plain body",
};

class FakeEventSource {
  onopen: ((ev: Event) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  addEventListener(): void {}
  close(): void {}
}

function inboxFetch(opts: { scopes?: string[]; markReadStatus?: number } = {}) {
  const scopes = opts.scopes ?? ["mail.read", "mail.write", "mail.admin", "mail.audit.read"];
  const items = [{ ...listItem }];
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method ?? "GET").toUpperCase();
    if (url.endsWith("/v1/session")) {
      return json(200, sessionView(scopes));
    }
    if (url.endsWith("/v1/status")) {
      return json(200, {
        ready: true,
        revisions: {},
        listeners: [],
        store: { messageCount: 1, storeBytes: 12, unreadCount: 2, storeGeneration: 1, epoch: 1 },
      });
    }
    if (url.startsWith("/v1/messages?") || url === "/v1/messages") {
      if (method === "DELETE") {
        items.length = 0;
        return json(200, { deleted: 1 });
      }
      return json(200, { revision: "r", storeGeneration: 1, items, nextCursor: null });
    }
    if (url.endsWith("/v1/messages/01JTEST:read") && method === "POST") {
      items[0] = { ...items[0]!, read: true };
      return new Response(null, { status: opts.markReadStatus ?? 204 });
    }
    if (url.includes("/v1/messages/01JTEST") && method === "GET") {
      return json(200, { ...fullMessage, read: items[0]?.read ?? false });
    }
    return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
  });
}

describe("InboxPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("selects a message with GET then CSRF mark-read POST", async () => {
    const user = userEvent.setup();
    const fetchMock = inboxFetch();
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("EventSource", FakeEventSource);

    renderApp(
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<InboxPage />} />
        </Route>
      </Routes>,
    );

    expect(await screen.findByText("App")).toBeInTheDocument();
    expect(screen.getByText("Your lab sign-in code is 482193")).toBeInTheDocument();
    expect(screen.getByText("1 attachment")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /relay/i })).toBeNull();

    await user.click(screen.getByRole("button", { name: /App/i }));

    expect(await screen.findByRole("heading", { name: /Your lab sign-in code/i })).toBeInTheDocument();
    expect(screen.getByTitle("HTML preview")).toHaveAttribute("sandbox", "");
    expect(screen.getByText(/sandbox empty/i)).toBeInTheDocument();

    await waitFor(() => {
      const posts = fetchMock.mock.calls.filter((c) => String(c[0]).includes(":read") && (c[1]?.method ?? "GET").toUpperCase() === "POST");
      expect(posts).toHaveLength(1);
      expect(new Headers(posts[0]?.[1]?.headers).get(CSRF_HEADER)).toBe("csrf-test");
    });
    expect(fetchMock.mock.calls.some((c) => String(c[0]).includes("markRead=true"))).toBe(false);
    await waitFor(() => {
      expect(screen.queryByText("2")).toBeNull();
    });
  });

  it("does not POST mark-read without write scope", async () => {
    const user = userEvent.setup();
    const fetchMock = inboxFetch({ scopes: ["mail.read"] });
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("EventSource", FakeEventSource);

    renderApp(
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<InboxPage />} />
        </Route>
      </Routes>,
    );

    await user.click(await screen.findByRole("button", { name: /App/i }));
    await screen.findByRole("heading", { name: /Your lab sign-in code/i });
    expect(fetchMock.mock.calls.some((c) => String(c[0]).includes(":read"))).toBe(false);
  });

  it("clears the inbox with an in-page confirm", async () => {
    const user = userEvent.setup();
    const fetchMock = inboxFetch();
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("EventSource", FakeEventSource);
    const confirm = vi.spyOn(window, "confirm");

    renderApp(
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<InboxPage />} />
        </Route>
      </Routes>,
    );

    await user.click(await screen.findByRole("button", { name: /Clear inbox/i }));
    expect(confirm).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /^Clear inbox$/i }));
    await waitFor(() => {
      expect(fetchMock.mock.calls.some((c) => String(c[0]) === "/v1/messages" && (c[1]?.method ?? "").toUpperCase() === "DELETE")).toBe(true);
    });
  });
});
