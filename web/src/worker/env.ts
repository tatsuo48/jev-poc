import type { Budget } from "./budget";

export interface Env {
  ASSETS: Fetcher;
  BUDGET: DurableObjectNamespace<Budget>;
  /** Absent if the Rate Limiting binding is unavailable; the daily budget still caps the cost. */
  MOVE_LIMITER?: RateLimit;
  TYPESAFE_API_KEY: string;
  JEV_ENDPOINT: string;
  DAILY_LIMIT: string;
}

export const DEFAULT_DAILY_LIMIT = 20000;

export function dailyLimit(env: Env): number {
  const n = Number(env.DAILY_LIMIT);
  return Number.isInteger(n) && n > 0 ? n : DEFAULT_DAILY_LIMIT;
}
