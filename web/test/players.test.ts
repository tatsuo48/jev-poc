import { describe, expect, it } from "vitest";
import { type Board, Game } from "../src/shared/game";
import { ApiError, type Player, createPlayer, monotonicity, playerSeed } from "../src/client/players";

const stuck: Board = [
  [2, 4, 2, 4],
  [4, 2, 4, 2],
  [2, 4, 2, 4],
  [4, 2, 4, 2],
];
const cornerTile: Board = [
  [2, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
];

async function playOut(p: Player, seed: number): Promise<number> {
  const g = new Game(seed);
  while (!g.over()) g.step((await p.pick(g.board)).move);
  return g.score;
}

describe("classic players", () => {
  it("finish games with legal moves and reject a stuck board", async () => {
    for (const name of ["random", "greedy", "expectimax"] as const) {
      const p = createPlayer(name, 1);
      expect(p.usesJev).toBe(false);
      expect(await playOut(createPlayer(name, 3, undefined, 1), 3)).toBeGreaterThan(0);
      await expect(p.pick(stuck)).rejects.toThrow("no legal moves");
    }
  });

  it("random is deterministic per seed", async () => {
    expect(await playOut(createPlayer("random", 5), 9)).toBe(await playOut(createPlayer("random", 5), 9));
  });

  it("playerSeed decorrelates the random player's RNG from the game's own RNG", () => {
    expect(playerSeed(1)).not.toBe(1);
    expect(playerSeed(0)).toBe(0x9e3779b9);
  });

  it("greedy prefers the higher gain, then more empty cells, then ALL_MOVES order", async () => {
    const greedy = createPlayer("greedy", 0);
    const column: Board = [[2, 0, 0, 0], [4, 0, 0, 0], [4, 0, 0, 0], [8, 0, 0, 0]];
    expect((await greedy.pick(column)).move).toBe("up");
    const tie: Board = [[4, 0, 0, 0], [4, 2, 2, 0], [0, 0, 0, 0], [2, 0, 0, 2]];
    expect((await greedy.pick(tie)).move).toBe("left");
  });

  it("monotonicity is 0 for sorted lines and negative for zigzags", () => {
    const ordered: Board = [[64, 32, 16, 8], [32, 16, 8, 4], [16, 8, 4, 2], [8, 4, 2, 0]];
    expect(monotonicity(ordered)).toBeCloseTo(0);
    expect(monotonicity([[2, 8, 2, 8], [0, 0, 0, 0], [0, 0, 0, 0], [0, 0, 0, 0]])).toBeLessThan(0);
  });

  it("expectimax beats random over the same seeds", async () => {
    let ex = 0;
    let rnd = 0;
    for (const seed of [1, 2, 3]) {
      ex += await playOut(createPlayer("expectimax", seed, undefined, 2), seed);
      rnd += await playOut(createPlayer("random", seed), seed);
    }
    expect(ex).toBeGreaterThan(rnd);
  });
});

describe("jev player", () => {
  const okFetch = (body: unknown, seen: { url?: string; init?: RequestInit } = {}) =>
    (async (url: RequestInfo | URL, init?: RequestInit) => {
      seen.url = String(url);
      seen.init = init;
      return new Response(JSON.stringify(body), { status: 200 });
    }) as typeof fetch;

  it("posts the player and the board and returns the answer", async () => {
    const seen: { url?: string; init?: RequestInit } = {};
    const p = createPlayer("jev-sim", 1, okFetch({ move: "right", probabilities: { down: 0.3, right: 0.7 }, confidence: 0.4, remaining: 10 }, seen));
    expect(p.usesJev).toBe(true);
    const got = await p.pick(cornerTile);
    expect(got).toEqual({ move: "right", info: { probabilities: { down: 0.3, right: 0.7 }, confidence: 0.4, remaining: 10 } });
    expect(seen.url).toBe("/api/move");
    expect(seen.init!.method).toBe("POST");
    expect(JSON.parse(seen.init!.body as string)).toEqual({ player: "jev-sim", board: cornerTile });
  });

  it("does not call the API when only one move is legal", async () => {
    let calls = 0;
    const p = createPlayer("jev-raw", 1, (async () => { calls++; return new Response("{}"); }) as typeof fetch);
    const got = await p.pick([[2, 4, 2, 4], [0, 0, 0, 0], [0, 0, 0, 0], [0, 0, 0, 0]]);
    expect(got).toEqual({ move: "down", info: {} });
    expect(calls).toBe(0);
  });

  it("turns error responses into ApiError", async () => {
    const p = createPlayer("jev-raw", 1, (async () => new Response('{"error":"daily_budget_exhausted"}', { status: 429 })) as typeof fetch);
    const err = await p.pick(cornerTile).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("daily_budget_exhausted");
    expect(err.status).toBe(429);
  });

  it("reports network failures and illegal answers as ApiError", async () => {
    const down = createPlayer("jev-raw", 1, (async () => { throw new TypeError("offline"); }) as typeof fetch);
    expect((await down.pick(cornerTile).catch((e) => e)).code).toBe("network_error");
    const bad = createPlayer("jev-raw", 1, okFetch({ move: "up" }));
    expect((await bad.pick(cornerTile).catch((e) => e)).code).toBe("upstream_error");
  });

  it("maps a non-JSON 429 page (Cloudflare's own) to rate_limited", async () => {
    const p = createPlayer("jev-raw", 1, (async () => new Response("<html>Too Many Requests</html>", { status: 429 })) as typeof fetch);
    const err = await p.pick(cornerTile).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("rate_limited");
    expect(err.status).toBe(429);
  });

  it("maps a non-JSON 5xx page (Cloudflare's own) to service_unavailable", async () => {
    const p = createPlayer("jev-raw", 1, (async () => new Response("<html>Service Unavailable</html>", { status: 503 })) as typeof fetch);
    const err = await p.pick(cornerTile).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("service_unavailable");
    expect(err.status).toBe(503);
  });

  it("a JSON error body wins over the status-based mapping", async () => {
    const p = createPlayer("jev-raw", 1, (async () => new Response('{"error":"daily_budget_exhausted"}', { status: 500 })) as typeof fetch);
    const err = await p.pick(cornerTile).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe("daily_budget_exhausted");
  });
});
