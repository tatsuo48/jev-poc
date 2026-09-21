import { describe, expect, it } from "vitest";
import type { Move } from "../src/shared/game";
import { buildPrompt } from "../src/shared/prompt";
import { UpstreamError, askJev } from "../src/worker/jev";

const board = [
  [2, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
];
const legal: Move[] = ["down", "right"];
const prompt = buildPrompt("jev-raw", board, legal);

const ok = (answer: unknown) =>
  new Response(JSON.stringify({ model: "jev-1.13.0", answers: { move: answer }, usage: {} }), { status: 200 });

describe("askJev", () => {
  it("sends one choice question with the bearer key and parses the answer", async () => {
    let seen: { url: string; init: RequestInit } | undefined;
    const answer = await askJev(
      async (url, init) => {
        seen = { url, init };
        return ok({ type: "choice", choice: "right", probabilities: { down: 0.3, right: 0.7, up: 9 }, confidence: 0.4 });
      },
      "https://jev.example/v1",
      "test-key",
      prompt,
      legal,
    );

    expect(answer).toEqual({ move: "right", probabilities: { down: 0.3, right: 0.7 }, confidence: 0.4 });
    expect(seen!.url).toBe("https://jev.example/v1");
    expect(seen!.init.method).toBe("POST");
    const headers = new Headers(seen!.init.headers);
    expect(headers.get("Authorization")).toBe("Bearer test-key");
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(seen!.init.signal).toBeInstanceOf(AbortSignal);
    expect(seen!.init.redirect).toBe("manual");
    const body = JSON.parse(seen!.init.body as string);
    expect(body.model).toBe("jev-latest");
    expect(body.state).toEqual({ board });
    expect(body.questions.move.type).toBe("choice");
    expect(Object.keys(body.questions.move.criteria).sort()).toEqual(["down", "right"]);
  });

  it("reports a non-200 status without leaking the body or the key", async () => {
    const err = await askJev(
      async () => new Response("echo: Bearer test-key", { status: 401 }),
      "https://jev.example/v1", "test-key", prompt, legal,
    ).catch((e) => e);
    expect(err).toBeInstanceOf(UpstreamError);
    expect(err.status).toBe(401);
    expect(err.reason).toBe("status");
    expect(String(err.message)).not.toContain("test-key");
    expect(String(err.message)).not.toContain("echo");
  });

  it("never follows a redirect (and so never forwards the bearer key to it)", async () => {
    const err = await askJev(
      async () => new Response(null, { status: 302, headers: { Location: "https://evil.example/steal" } }),
      "https://jev.example/v1", "test-key", prompt, legal,
    ).catch((e) => e);
    expect(err).toBeInstanceOf(UpstreamError);
    expect(err.status).toBe(302);
    expect(err.reason).toBe("status");
  });

  it("reports network failures", async () => {
    const err = await askJev(
      async () => { throw new TypeError("connection reset"); },
      "https://jev.example/v1", "test-key", prompt, legal,
    ).catch((e) => e);
    expect(err).toBeInstanceOf(UpstreamError);
    expect(err.reason).toBe("network");
  });

  it.each([
    ["an illegal choice", { type: "choice", choice: "up", probabilities: {}, confidence: 1 }],
    ["an unknown choice", { type: "choice", choice: "sideways" }],
    ["a missing answer", undefined],
  ])("rejects %s", async (_name, answer) => {
    const err = await askJev(async () => ok(answer), "https://jev.example/v1", "test-key", prompt, legal).catch((e) => e);
    expect(err).toBeInstanceOf(UpstreamError);
    expect(err.reason).toBe("bad_answer");
  });

  it("rejects a body that is not JSON", async () => {
    const err = await askJev(async () => new Response("<html>", { status: 200 }), "https://jev.example/v1", "k", prompt, legal).catch((e) => e);
    expect(err).toBeInstanceOf(UpstreamError);
    expect(err.reason).toBe("bad_answer");
  });
});
