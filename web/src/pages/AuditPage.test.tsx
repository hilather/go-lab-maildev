import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { AuditPage } from "./AuditPage";

describe("AuditPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("shows a muted empty state", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/audit")) {
          return json(200, { events: [] });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
      }),
    );
    renderApp(<AuditPage />, { route: "/audit" });
    const empty = await screen.findByText(/no audit events/i);
    expect(empty).toHaveClass("empty-state");
    expect(screen.getByRole("heading", { name: /^audit$/i })).toHaveClass("page-title");
  });

  it("renders table headers for a row", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/audit")) {
          return json(200, {
            events: [
              {
                id: "evt-1",
                time: "2026-08-29T12:00:00Z",
                actorId: "admin",
                transport: "rest",
                capability: "state.reset",
                result: "ok",
                messageId: "01JTEST",
              },
            ],
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labmail:error:not-found" });
      }),
    );
    renderApp(<AuditPage />, { route: "/audit" });
    expect(await screen.findByRole("columnheader", { name: /^time$/i })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: /^capability$/i })).toBeInTheDocument();
    expect(screen.getByText("state.reset")).toBeInTheDocument();
    expect(screen.getByText("2026-08-29T12:00:00Z")).toBeInTheDocument();
    expect(document.querySelector(".panel")).not.toBeNull();
  });
});
