import { describe, expect, it } from "vitest";
import { canSubmitReset, containsForbiddenControl, navItems } from "./forbidden";

describe("operator nav", () => {
  it("has no relay, send, compose, or outgoing controls", () => {
    const labels = navItems(true, true).map((i) => i.label);
    expect(containsForbiddenControl(labels)).toBe(false);
    expect(labels).toEqual(["Inbox", "Status", "Audit", "Reset"]);
    expect(navItems(false, false).map((i) => i.label)).toEqual(["Inbox", "Status"]);
  });

  it("gates reset on the exact phrase and confirmation", () => {
    expect(canSubmitReset("RESET", true, true)).toBe(true);
    expect(canSubmitReset("reset", true, true)).toBe(false);
    expect(canSubmitReset("RESET", false, true)).toBe(false);
    expect(canSubmitReset("RESET", true, false)).toBe(false);
  });
});
