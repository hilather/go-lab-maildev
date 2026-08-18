import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
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
  read: false,
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
    const user = userEvent.setup();
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/v1/session")) {
        return json(200, sessionView());
      }
      if (url.includes("/v1/messages/01JTEST") && !url.includes("/raw")) {
        return json(200, message);
      }
      return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp(
      <Routes>
        <Route path="/messages/:id" element={<MessagePage />} />
      </Routes>,
      { route: "/messages/01JTEST" },
    );
    expect(await screen.findByRole("heading", { name: "Hello" })).toBeInTheDocument();
    expect(screen.getByText("plain body")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /relay/i })).toBeNull();

    await user.click(screen.getByRole("tab", { name: /HTML preview/i }));
    const frame = await screen.findByTitle("HTML preview");
    expect(frame).toHaveAttribute("src", "/v1/messages/01JTEST/preview");
    expect(frame.getAttribute("sandbox")).toBe(PREVIEW_SANDBOX);
    expect(isSafePreviewSandbox(frame.getAttribute("sandbox") ?? "missing")).toBe(true);
    await waitFor(() => {
      expect(fetchMock.mock.calls.some((c) => String(c[0]).includes("/v1/messages/01JTEST"))).toBe(true);
    });
  });
});
