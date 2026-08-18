import { describe, expect, it } from "vitest";
import { PREVIEW_SANDBOX, isSafePreviewSandbox, sandboxTokens } from "./sandbox";

describe("preview sandbox", () => {
  it("ships an empty sandbox with no script or same-origin tokens", () => {
    expect(PREVIEW_SANDBOX).toBe("");
    expect(sandboxTokens(PREVIEW_SANDBOX)).toEqual([]);
    expect(isSafePreviewSandbox(PREVIEW_SANDBOX)).toBe(true);
    expect(isSafePreviewSandbox("allow-scripts")).toBe(false);
    expect(isSafePreviewSandbox("allow-same-origin")).toBe(false);
    expect(isSafePreviewSandbox("allow-popups-to-escape-sandbox")).toBe(false);
  });
});
