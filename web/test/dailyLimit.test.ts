import { describe, expect, it } from "vitest";
import { DEFAULT_DAILY_LIMIT, dailyLimit } from "../src/worker/dailyLimit";

// dailyLimit lives in its own import-free module (rather than in env.ts, which pulls in
// @cloudflare/workers-types via the Env interface) specifically so it can be unit tested
// here without the Workers runtime types.
describe("dailyLimit", () => {
  it("parses a plain integer string", () => {
    expect(dailyLimit("20000")).toBe(20000);
  });

  it("accepts 0 as a valid kill switch", () => {
    expect(dailyLimit("0")).toBe(0);
  });

  it("falls back to the default when empty", () => {
    expect(dailyLimit("")).toBe(DEFAULT_DAILY_LIMIT);
  });

  it("falls back to the default when undefined", () => {
    expect(dailyLimit(undefined)).toBe(DEFAULT_DAILY_LIMIT);
  });

  it("falls back to the default for non-numeric text", () => {
    expect(dailyLimit("abc")).toBe(DEFAULT_DAILY_LIMIT);
  });

  it("falls back to the default for a negative number", () => {
    expect(dailyLimit("-5")).toBe(DEFAULT_DAILY_LIMIT);
  });

  it("falls back to the default for a fractional number", () => {
    expect(dailyLimit("1.5")).toBe(DEFAULT_DAILY_LIMIT);
  });

  it("trims surrounding whitespace", () => {
    expect(dailyLimit(" 300 ")).toBe(300);
  });
});
