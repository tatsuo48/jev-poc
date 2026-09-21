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
  /** False when TYPESAFE_API_KEY is not configured, so jev must never be called. */
  hasApiKey: boolean;
  log(message: string): void;
}

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" },
  });
}

const fail = (status: number, error: string) => json(status, { error });

/**
 * Reads the body without ever buffering more than `limit` bytes. As soon as the running
 * total exceeds the limit the underlying stream is cancelled and null is returned, so an
 * attacker cannot force the Worker to fully buffer an oversized body before rejecting it.
 */
async function readBodyLimited(request: Request, limit: number): Promise<string | null> {
  if (!request.body) return "";
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > limit) {
      await reader.cancel();
      return null;
    }
    chunks.push(value);
  }
  const buf = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    buf.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return new TextDecoder().decode(buf);
}

/**
 * Handles /api/*. Returns null for every other path so the caller can serve static assets.
 * Any exception not already handled by a more specific case becomes a generic 500 so the
 * Workers runtime's own error page (which could leak implementation details) is never sent.
 */
export async function handleApi(request: Request, deps: Deps): Promise<Response | null> {
  try {
    const url = new URL(request.url);
    if (!url.pathname.startsWith("/api/")) return null;

    if (url.pathname === "/api/status") {
      if (request.method !== "GET") return fail(405, "method_not_allowed");
      let remaining: number;
      try {
        remaining = await deps.peekBudget();
      } catch {
        deps.log("budget unavailable");
        return fail(503, "service_unavailable");
      }
      return json(200, { remaining, limit: deps.dailyLimit });
    }
    if (url.pathname !== "/api/move") return fail(404, "not_found");
    return await handleMove(request, url, deps);
  } catch {
    // Never log or return the exception text: it could leak implementation details.
    deps.log("unexpected error");
    return fail(500, "internal_error");
  }
}

async function handleMove(request: Request, url: URL, deps: Deps): Promise<Response> {
  if (request.method !== "POST") return fail(405, "method_not_allowed");
  if (!(request.headers.get("Content-Type") ?? "").includes("application/json")) {
    return fail(415, "unsupported_media_type");
  }
  if (request.headers.get("Origin") !== url.origin) return fail(403, "forbidden_origin");

  const contentLength = request.headers.get("Content-Length");
  if (contentLength !== null) {
    const declared = Number(contentLength);
    if (Number.isFinite(declared) && declared > MAX_BODY_BYTES) return fail(413, "payload_too_large");
  }

  const text = await readBodyLimited(request, MAX_BODY_BYTES);
  if (text === null) return fail(413, "payload_too_large");
  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    return fail(400, "invalid_request");
  }
  const parsed = parseMoveRequest(body);
  if (!parsed) return fail(400, "invalid_request");

  if (!deps.hasApiKey) {
    deps.log("api key not configured");
    return fail(503, "service_unavailable");
  }

  const ip = request.headers.get("CF-Connecting-IP") ?? "unknown";
  let allowed: boolean;
  try {
    allowed = await deps.allowRequest(ip);
  } catch {
    // Fail open: the daily budget still caps the cost even if the rate limiter is down.
    deps.log("rate limiter failed");
    allowed = true;
  }
  if (!allowed) return fail(429, "rate_limited");

  let budget: { ok: boolean; remaining: number };
  try {
    budget = await deps.takeBudget();
  } catch {
    // Fail closed: without the budget counter we cannot bound the cost, so refuse to call jev.
    deps.log("budget unavailable");
    return fail(503, "service_unavailable");
  }
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
