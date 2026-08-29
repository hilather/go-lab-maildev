import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { MessagePage } from "./MessagePage";
import { PREVIEW_SANDBOX, isSafePreviewSandbox } from "../ui/sandbox";

const message = {
  id: "01JTEST",
  receivedAt: "2026-08-18T00:00:00Z",
  subject: "Hello",
  from: [{ name: "Alice", address: "alice@lab.test" }],
  to: [{ name: "", address: "bob@lab.test" }],
  cc: [],
  bcc: [],
  messageId: "<1@lab>",
  read: true,
  size: 12,
  hasHTML: true,
  envelope: { from: "alice@lab.test", to: ["bob@lab.test"], helo: "lab", remoteAddress: "127.0.0.1:1", tls: false },
  headers: [{ name: "Subject", value: "Hello" }],
  attachments: [{ id: "att1", filename: "note.txt", contentType: "text/plain", size: 4, checksum: "ab" }],
  text: "plain body",
};

describe("MessagePage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("loads a message and previews HTML in an empty sandbox iframe", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/v1/session")) {
        return json(200, sessionView());
      }
      if (url.includes("/v1/messages/01JTEST") && !url.includes("/raw") && !url.includes(":read")) {
        return json(200, message);
      }
      return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp(<MessagePage messageId="01JTEST" embedded />);
    expect(await screen.findByRole("heading", { name: "Hello" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /relay/i })).toBeNull();

    const frame = await screen.findByTitle("HTML preview");
    expect(frame).toHaveAttribute("src", "/v1/messages/01JTEST/preview");
    expect(frame.hasAttribute("sandbox")).toBe(true);
    expect(frame.getAttribute("sandbox")).toBe("");
    expect(frame.getAttribute("sandbox")).toBe(PREVIEW_SANDBOX);
    expect(isSafePreviewSandbox(frame.getAttribute("sandbox"))).toBe(true);
    expect(screen.getByText(/sandbox empty · img-src data: only · no remote pixels/i)).toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock.mock.calls.some((c) => String(c[0]).includes("/v1/messages/01JTEST"))).toBe(true);
    });
    expect(fetchMock.mock.calls.some((c) => String(c[0]).includes("markRead=true"))).toBe(false);
    expect(fetchMock.mock.calls.some((c) => String(c[0]).includes(":read"))).toBe(false);
  });

  it("deletes with an in-page confirm, not window.confirm", async () => {
    const user = userEvent.setup();
    const onDeleted = vi.fn();
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = (init?.method ?? "GET").toUpperCase();
      if (url.endsWith("/v1/session")) {
        return json(200, sessionView());
      }
      if (url.includes("/v1/messages/01JTEST") && method === "DELETE") {
        return new Response(null, { status: 204 });
      }
      if (url.includes("/v1/messages/01JTEST") && !url.includes(":read")) {
        return json(200, message);
      }
      return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
    });
    vi.stubGlobal("fetch", fetchMock);
    const confirm = vi.spyOn(window, "confirm");

    renderApp(<MessagePage messageId="01JTEST" embedded onDeleted={onDeleted} />);
    await user.click(await screen.findByRole("button", { name: /^Delete$/i }));
    expect(confirm).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: /^Delete$/i }));
    await waitFor(() => {
      expect(onDeleted).toHaveBeenCalled();
    });
  });
});
