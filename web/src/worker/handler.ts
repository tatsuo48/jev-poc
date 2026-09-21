import type { Move } from "../shared/game";
import { type Prompt, buildPrompt } from "../shared/prompt";
import { type JevAnswer, UpstreamError } from "./jev";
import { parseMoveRequest } from "./validate";

const MAX_BODY_BYTES = 2048;

/** Everything the handler needs from the outside world, so it can be tested without the Workers runtime. */
export interface Deps {
  askJev(prompt: Prompt, legal: Move[]): Promise<JevAnswer>;
  takeBudget(): Promise<{ ok: boolean; remaining: number }>;
  peekBudget(): Promise<number>;
  allowRequest(ip: string): Promise<boolean>;
  dailyLimit: number;
  log(message: string): void;
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", "Cache-Control": "no-store" },
  });
}

const fail = (status: number, error: string) => json(status, { error });

/** Handles /api/*. Returns null for every other path so the caller can serve static assets. */
export async function handleApi(request: Request, deps: Deps): Promise<Response | null> {
  const url = new URL(request.url);
  if (!url.pathname.startsWith("/api/")) return null;

  if (url.pathname === "/api/status") {
    if (request.method !== "GET") return fail(405, "method_not_allowed");
    return json(200, { remaining: await deps.peekBudget(), limit: deps.dailyLimit });
  }
  if (url.pathname !== "/api/move") return fail(404, "not_found");
  return handleMove(request, url, deps);
}

async function handleMove(request: Request, url: URL, deps: Deps): Promise<Response> {
  if (request.method !== "POST") return fail(405, "method_not_allowed");
  if (!(request.headers.get("Content-Type") ?? "").includes("application/json")) {
    return fail(415, "unsupported_media_type");
  }
  if (request.headers.get("Origin") !== url.origin) return fail(403, "forbidden_origin");

  const text = await request.text();
  if (new TextEncoder().encode(text).length > MAX_BODY_BYTES) return fail(413, "payload_too_large");
  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    return fail(400, "invalid_request");
  }
  const parsed = parseMoveRequest(body);
  if (!parsed) return fail(400, "invalid_request");

  const ip = request.headers.get("CF-Connecting-IP") ?? "unknown";
  if (!(await deps.allowRequest(ip))) return fail(429, "rate_limited");

  const budget = await deps.takeBudget();
  if (!budget.ok) return fail(429, "daily_budget_exhausted");

  try {
    const answer = await deps.askJev(buildPrompt(parsed.player, parsed.board, parsed.legal), parsed.legal);
    return json(200, { ...answer, remaining: budget.remaining });
  } catch (err) {
    // Never log or return upstream text: it could echo the API key.
    deps.log(err instanceof UpstreamError ? err.message : "jev call failed");
    return fail(502, "upstream_error");
  }
}
