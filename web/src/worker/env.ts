import type { Budget } from "./budget";

export { DEFAULT_DAILY_LIMIT, dailyLimit } from "./dailyLimit";

export interface Env {
  ASSETS: Fetcher;
  BUDGET: DurableObjectNamespace<Budget>;
  /** Absent if the Rate Limiting binding is unavailable; the daily budget still caps the cost. */
  MOVE_LIMITER?: RateLimit;
  TYPESAFE_API_KEY: string;
  JEV_ENDPOINT: string;
  DAILY_LIMIT: string;
}
