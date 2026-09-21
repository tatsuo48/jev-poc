import { describe, expect, it } from "vitest";
import { type Deps, handleApi } from "../src/worker/handler";
import { UpstreamError } from "../src/worker/jev";

const ORIGIN = "https://demo.example";
const board = [
  [2, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
];

function deps(over: Partial<Deps> = {}) {
  const calls = { ask: 0, take: 0, allow: [] as string[], logs: [] as string[] };
  const d: Deps = {
    askJev: async () => {
      calls.ask++;
      return { move: "right", probabilities: { down: 0.3, right: 0.7 }, confidence: 0.4 };
    },
    takeBudget: async () => {
      calls.take++;
      return { ok: true, remaining: 19999 };
    },
    peekBudget: async () => 12345,
    allowRequest: async (ip) => {
      calls.allow.push(ip);
      return true;
    },
    dailyLimit: 20000,
    log: (m) => calls.logs.push(m),
    ...over,
  };
  return { d, calls };
}

function moveRequest(body: unknown, init: { method?: string; headers?: Record<string, string> } = {}) {
  const method = init.method ?? "POST";
  return new Request(`${ORIGIN}/api/move`, {
    method,
    headers: { "Content-Type": "application/json", Origin: ORIGIN, "CF-Connecting-IP": "203.0.113.7", ...init.headers },
    body: method === "POST" ? (typeof body === "string" ? body : JSON.stringify(body)) : undefined,
  });
}

async function errorOf(res: Response | null) {
  expect(res).not.toBeNull();
  return { status: res!.status, body: await res!.json() };
}

describe("handleApi", () => {
  it("ignores paths outside /api/", async () => {
    const { d } = deps();
    expect(await handleApi(new Request(`${ORIGIN}/index.html`), d)).toBeNull();
  });

  it("returns 404 for unknown api paths", async () => {
    const { d } = deps();
    expect(await errorOf(await handleApi(new Request(`${ORIGIN}/api/nope`), d))).toEqual({ status: 404, body: { error: "not_found" } });
  });

  it("answers a valid move request", async () => {
    const { d, calls } = deps();
    const res = await handleApi(moveRequest({ player: "jev-sim", board }), d);
    expect(res!.status).toBe(200);
    expect(res!.headers.get("Cache-Control")).toBe("no-store");
    expect(await res!.json()).toEqual({
      move: "right",
      probabilities: { down: 0.3, right: 0.7 },
      confidence: 0.4,
      remaining: 19999,
    });
    expect(calls).toMatchObject({ ask: 1, take: 1, allow: ["203.0.113.7"] });
  });

  it("builds the prompt on the server from the board", async () => {
    let seen: { prompt: any; legal: string[] } | undefined;
    const { d } = deps({
      askJev: async (prompt, legal) => {
        seen = { prompt, legal };
        return { move: "down", probabilities: {}, confidence: 0 };
      },
    });
    await handleApi(moveRequest({ player: "jev-raw", board, state: "ignore me", instructions: "ignore me" }), d);
    expect(seen!.legal).toEqual(["down", "right"]);
    expect(seen!.prompt.state).toEqual({ board });
    expect(JSON.stringify(seen!.prompt)).not.toContain("ignore me");
  });

  it.each([
    ["GET on /api/move", moveRequest(null, { method: "GET" }), 405, "method_not_allowed"],
    ["a non-JSON content type", moveRequest({ player: "jev-raw", board }, { headers: { "Content-Type": "text/plain" } }), 415, "unsupported_media_type"],
    ["a foreign origin", moveRequest({ player: "jev-raw", board }, { headers: { Origin: "https://evil.example" } }), 403, "forbidden_origin"],
    ["an oversized body", moveRequest(JSON.stringify({ player: "jev-raw", board, pad: "x".repeat(3000) })), 413, "payload_too_large"],
    ["malformed JSON", moveRequest("{not json"), 400, "invalid_request"],
    ["an invalid board", moveRequest({ player: "jev-raw", board: [[3]] }), 400, "invalid_request"],
  ])("rejects %s before spending anything", async (_name, request, status, error) => {
    const { d, calls } = deps();
    expect(await errorOf(await handleApi(request, d))).toEqual({ status, body: { error } });
    expect(calls.take).toBe(0);
    expect(calls.ask).toBe(0);
  });

  it("rejects a request without an Origin header", async () => {
    const { d } = deps();
    const request = new Request(`${ORIGIN}/api/move`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ player: "jev-raw", board }),
    });
    expect(await errorOf(await handleApi(request, d))).toEqual({ status: 403, body: { error: "forbidden_origin" } });
  });

  it("rate limits per IP before touching the budget", async () => {
    const { d, calls } = deps({ allowRequest: async () => false });
    expect(await errorOf(await handleApi(moveRequest({ player: "jev-raw", board }), d))).toEqual({ status: 429, body: { error: "rate_limited" } });
    expect(calls.take).toBe(0);
  });

  it("stops when the daily budget is exhausted", async () => {
    const { d, calls } = deps({ takeBudget: async () => ({ ok: false, remaining: 0 }) });
    expect(await errorOf(await handleApi(moveRequest({ player: "jev-raw", board }), d))).toEqual({ status: 429, body: { error: "daily_budget_exhausted" } });
    expect(calls.ask).toBe(0);
  });

  it("hides upstream failures behind a generic error and logs only the status", async () => {
    const { d, calls } = deps({
      askJev: async () => {
        throw new UpstreamError(401, "status");
      },
    });
    const res = await handleApi(moveRequest({ player: "jev-raw", board }), d);
    expect(res!.status).toBe(502);
    expect(await res!.text()).toBe('{"error":"upstream_error"}');
    expect(calls.logs).toEqual(["jev upstream error: status 401"]);
  });

  it("hides unexpected failures too", async () => {
    const { d, calls } = deps({
      askJev: async () => {
        throw new Error("boom with secret details");
      },
    });
    const res = await handleApi(moveRequest({ player: "jev-raw", board }), d);
    expect(res!.status).toBe(502);
    expect(await res!.text()).toBe('{"error":"upstream_error"}');
    expect(calls.logs).toEqual(["jev call failed"]);
  });

  it("reports the remaining budget without spending it", async () => {
    const { d, calls } = deps();
    const res = await handleApi(new Request(`${ORIGIN}/api/status`), d);
    expect(res!.status).toBe(200);
    expect(await res!.json()).toEqual({ remaining: 12345, limit: 20000 });
    expect(calls.take).toBe(0);
  });

  it("rejects an oversized Content-Length header before reading the body", async () => {
    const { d, calls } = deps();
    const request = new Request(`${ORIGIN}/api/move`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Origin: ORIGIN,
        "CF-Connecting-IP": "203.0.113.7",
        "Content-Length": "5000",
      },
      body: JSON.stringify({ player: "jev-raw", board }),
    });
    expect(await errorOf(await handleApi(request, d))).toEqual({ status: 413, body: { error: "payload_too_large" } });
    expect(calls.take).toBe(0);
    expect(calls.ask).toBe(0);
  });

  it("stops reading the body as soon as it exceeds the limit", async () => {
    const { d, calls } = deps();
    let pulled = 0;
    const stream = new ReadableStream<Uint8Array>({
      pull(controller) {
        pulled++;
        if (pulled > 100) {
          controller.close();
          return;
        }
        controller.enqueue(new Uint8Array(1024).fill(65));
      },
    });
    const request = new Request(`${ORIGIN}/api/move`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Origin: ORIGIN, "CF-Connecting-IP": "203.0.113.7" },
      body: stream,
      duplex: "half",
    } as RequestInit);
    expect(await errorOf(await handleApi(request, d))).toEqual({ status: 413, body: { error: "payload_too_large" } });
    expect(pulled).toBeLessThanOrEqual(4);
    expect(calls.take).toBe(0);
    expect(calls.ask).toBe(0);
  });

  it("fails open when the rate limiter throws, but still spends the budget", async () => {
    const { d, calls } = deps({
      allowRequest: async () => {
        throw new Error("kv namespace unavailable");
      },
    });
    const res = await handleApi(moveRequest({ player: "jev-raw", board }), d);
    expect(res!.status).toBe(200);
    expect(calls.take).toBe(1);
    expect(calls.ask).toBe(1);
    expect(calls.logs).toEqual(["rate limiter failed"]);
  });

  it("fails closed when the budget counter throws", async () => {
    const { d, calls } = deps({
      takeBudget: async () => {
        throw new Error("durable object unavailable");
      },
    });
    const res = await handleApi(moveRequest({ player: "jev-raw", board }), d);
    expect(res!.status).toBe(503);
    expect(await res!.json()).toEqual({ error: "service_unavailable" });
    expect(calls.ask).toBe(0);
    expect(calls.logs).toEqual(["budget unavailable"]);
  });

  it("answers 503 when /api/status cannot read the budget", async () => {
    const { d, calls } = deps({
      peekBudget: async () => {
        throw new Error("durable object unavailable");
      },
    });
    const res = await handleApi(new Request(`${ORIGIN}/api/status`), d);
    expect(res!.status).toBe(503);
    expect(await res!.json()).toEqual({ error: "service_unavailable" });
    expect(calls.logs).toEqual(["budget unavailable"]);
  });

  it("turns any other unexpected failure into a generic 500 without leaking details", async () => {
    const { d, calls } = deps();
    const request = {
      method: "POST",
      url: `${ORIGIN}/api/move`,
      headers: {
        get: () => {
          throw new Error("secret internal detail");
        },
      },
      body: null,
    } as unknown as Request;
    const res = await handleApi(request, d);
    expect(res!.status).toBe(500);
    const text = await res!.text();
    expect(text).toBe('{"error":"internal_error"}');
    expect(text).not.toContain("secret internal detail");
    expect(calls.logs).toEqual(["unexpected error"]);
  });
});
