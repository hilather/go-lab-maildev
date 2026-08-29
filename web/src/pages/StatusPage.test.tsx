import { screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { StatusPage } from "./StatusPage";

function statusBody() {
  return {
    ready: true,
    revisions: { config: "abc" },
    listeners: [{ name: "smtp", address: ":1025" }],
    store: { messageCount: 3, storeBytes: 512, unreadCount: 1, storeGeneration: 9, epoch: 2 },
  };
}

describe("StatusPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders store stats in a panel", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/status")) {
          return json(200, statusBody());
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
      }),
    );
    renderApp(<StatusPage />, { route: "/status" });
    expect(await screen.findByRole("heading", { name: /^status$/i })).toHaveClass("page-title");
    expect(await screen.findByText("Messages")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("512 B")).toBeInTheDocument();
    expect(screen.getByText(":1025")).toBeInTheDocument();
    expect(document.querySelector(".panel")).not.toBeNull();
    expect(screen.getByText("Store")).toHaveClass("section-label");
  });

  it("shows an error banner when status fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/status")) {
          return json(500, {
            status: 500,
            title: "internal",
            detail: "status unavailable",
            code: "internal_error",
            type: "urn:labmail:error:internal-error",
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
      }),
    );
    renderApp(<StatusPage />, { route: "/status" });
    expect(await screen.findByRole("alert")).toHaveTextContent(/status unavailable/i);
  });

  it("shows a muted loading state before status arrives", async () => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/status")) {
          await gate;
          return json(200, statusBody());
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
      }),
    );
    renderApp(<StatusPage />, { route: "/status" });
    expect(await screen.findByRole("status")).toHaveClass("empty-state");
    expect(screen.getByRole("status")).toHaveTextContent(/loading status/i);
    release();
    await waitFor(() => {
      expect(screen.getByText("Messages")).toBeInTheDocument();
    });
  });
});
