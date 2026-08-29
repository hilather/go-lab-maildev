import { describe, expect, it } from "vitest";
import { formatRelativeReceived } from "./relativeTime";

describe("formatRelativeReceived", () => {
  const now = new Date(2026, 7, 29, 14, 30, 0);

  it("shows HH:MM on the same local day", () => {
    const received = new Date(2026, 7, 29, 11, 28, 0);
    expect(formatRelativeReceived(received.toISOString(), now)).toBe("11:28");
  });

  it("shows day and month later in the same year", () => {
    const received = new Date(2026, 6, 4, 10, 4, 0);
    expect(formatRelativeReceived(received.toISOString(), now)).toBe("4 Jul");
  });

  it("includes the year for older mail", () => {
    const received = new Date(2025, 11, 31, 23, 50, 0);
    expect(formatRelativeReceived(received.toISOString(), now)).toBe("31 Dec 2025");
  });
});
