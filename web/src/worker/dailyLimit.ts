/**
 * No imports on purpose: env.ts pulls in @cloudflare/workers-types through the Env
 * interface, which is unavailable to a plain vitest run. Keeping this parsing logic in its
 * own dependency-free module lets it be unit tested directly.
 */

/** Calls/day used when DAILY_LIMIT is unset, blank, or not a valid non-negative integer. */
export const DEFAULT_DAILY_LIMIT = 20000;

/**
 * Parses the DAILY_LIMIT env var. Blank or missing falls back to the default. Any
 * non-negative integer (including 0, which acts as a kill switch that stops every jev call)
 * is used as-is; anything else falls back to the default.
 */
export function dailyLimit(raw: string | undefined): number {
  const trimmed = (raw ?? "").trim();
  if (trimmed === "") return DEFAULT_DAILY_LIMIT;
  const n = Number(trimmed);
  return Number.isInteger(n) && n >= 0 ? n : DEFAULT_DAILY_LIMIT;
}
