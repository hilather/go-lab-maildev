import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState } from "../test/render";
import { containsForbiddenControl } from "../ui/forbidden";
import { LoginPage } from "./LoginPage";

const srcRoot = join(dirname(fileURLToPath(import.meta.url)), "..");

describe("remaining-page chrome", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("keeps approved tokens and no leftover paper/navy accent", () => {
    const css = readFileSync(join(srcRoot, "styles.css"), "utf8");
    expect(css).toMatch(/--accent:\s*#4aa384/);
    expect(css).toMatch(/--bg:\s*#0b0c0e/);
    expect(css).toMatch(/--panel:\s*#181a1f/);
    expect(css).not.toMatch(/#1f4b3a/);
    expect(css).not.toMatch(/button\[type="submit"\]/);
    expect(css).toMatch(/"IBM Plex Sans"/);
  });

  it("styles login as a panel with primary Sign in and no compose", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        json(401, {
          status: 401,
          title: "unauthenticated",
          detail: "authentication required",
          code: "unauthenticated",
          type: "urn:labmail:error:unauthenticated",
        }),
      ),
    );
    renderApp(<LoginPage />, { route: "/login" });
    expect(await screen.findByRole("heading", { name: /sign in to labmail/i })).toHaveClass("page-title");
    const form = document.querySelector("form");
    expect(form?.className.split(/\s+/)).toContain("panel");
    expect(screen.getByRole("button", { name: /sign in/i })).toHaveClass("primary");
    expect(containsForbiddenControl(["Sign in", "Bearer token", "Username and password"])).toBe(false);
  });
});
