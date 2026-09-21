import { describe, expect, it } from "vitest";
import { type Board, legalMoves } from "../src/shared/game";
import { GameLoop, type LoopEvents, type StepView } from "../src/client/loop";
import { ApiError, type Player } from "../src/client/players";

const noSleep = async () => {};

function recorder() {
  const log = { steps: [] as StepView[], retries: [] as number[], finished: [] as unknown[], errors: [] as string[] };
  const events: LoopEvents = {
    onStep: (s) => log.steps.push(s),
    onRetry: (n) => log.retries.push(n),
    onFinish: (r) => log.finished.push(r),
    onError: (c) => log.errors.push(c),
  };
  return { log, events };
}

function scripted(failures: (ApiError | null)[]): Player & { calls: number } {
  const p = {
    name: "jev-sim" as const,
    usesJev: true,
    calls: 0,
    async pick(board: Board) {
      const failure = failures[p.calls++] ?? null;
      if (failure) throw failure;
      return { move: legalMoves(board)[0], info: { confidence: 0.5, remaining: 7 } };
    },
  };
  return p;
}

describe("GameLoop", () => {
  it("plays to the end and reports every step", async () => {
    const { log, events } = recorder();
    const loop = new GameLoop(scripted([]), 4, 0, events, noSleep);
    await loop.start();
    expect(loop.running).toBe(false);
    expect(log.errors).toEqual([]);
    expect(log.steps.length).toBe(loop.game.moves);
    expect(log.steps.length).toBeGreaterThan(0);
    expect(log.steps[0].moves).toBe(1);
    expect(log.steps[0].info.remaining).toBe(7);
    expect(log.finished).toEqual([{ score: loop.game.score, moves: loop.game.moves, maxTile: expect.any(Number) }]);
  });

  it("retries rate limits and upstream errors with growing waits", async () => {
    const waits: number[] = [];
    const { log, events } = recorder();
    const player = scripted([new ApiError("rate_limited", 429), new ApiError("upstream_error", 502), null]);
    const loop = new GameLoop(player, 4, 0, events, async (ms) => { waits.push(ms); });
    await loop.start();
    expect(log.retries.slice(0, 2)).toEqual([1, 2]);
    expect(waits.filter((ms) => ms > 0).slice(0, 2)).toEqual([1000, 2000]);
    expect(log.errors).toEqual([]);
  });

  it("gives up after three retries, keeps the game, and can resume", async () => {
    const { log, events } = recorder();
    const fail = new ApiError("network_error", 0);
    const player = scripted([fail, fail, fail, fail]);
    const loop = new GameLoop(player, 4, 0, events, noSleep);
    await loop.start();
    expect(log.errors).toEqual(["network_error"]);
    expect(log.retries).toEqual([1, 2, 3]);
    expect(player.calls).toBe(4);
    expect(loop.game.moves).toBe(0);
    expect(loop.running).toBe(false);

    await loop.start();
    expect(loop.game.over()).toBe(true);
    expect(log.finished.length).toBe(1);
  });

  it("does not retry when the daily budget is exhausted or the request is invalid", async () => {
    for (const code of ["daily_budget_exhausted", "invalid_request", "forbidden_origin", "internal_error"]) {
      const { log, events } = recorder();
      const player = scripted([new ApiError(code, 429)]);
      await new GameLoop(player, 4, 0, events, noSleep).start();
      expect(log.errors).toEqual([code]);
      expect(log.retries).toEqual([]);
      expect(player.calls).toBe(1);
    }
  });

  it("retries a service_unavailable failure and continues the game", async () => {
    const { log, events } = recorder();
    const player = scripted([new ApiError("service_unavailable", 503), null]);
    const loop = new GameLoop(player, 4, 0, events, noSleep);
    await loop.start();
    expect(log.retries).toEqual([1]);
    expect(log.errors).toEqual([]);
    expect(log.steps.length).toBeGreaterThan(0);
  });

  it("never substitutes a move for a failed pick", async () => {
    const { log, events } = recorder();
    const fail = new ApiError("upstream_error", 502);
    const loop = new GameLoop(scripted([fail, fail, fail, fail]), 4, 0, events, noSleep);
    await loop.start();
    expect(log.steps).toEqual([]);
  });

  it("stop halts the loop after applying the pick that was already in flight", async () => {
    const { log, events } = recorder();
    let loop!: GameLoop;
    const player: Player = {
      name: "greedy",
      usesJev: false,
      async pick(board) {
        if (loop.game.moves === 3) loop.stop();
        return { move: legalMoves(board)[0], info: {} };
      },
    };
    loop = new GameLoop(player, 4, 0, events, noSleep);
    await loop.start();
    expect(loop.game.moves).toBe(4);
    expect(log.steps.length).toBe(4);
    expect(log.finished).toEqual([]);
    expect(loop.running).toBe(false);
  });

  it("stop during the first backoff wait ends the loop silently without another pick", async () => {
    const { log, events } = recorder();
    let loop!: GameLoop;
    const player = scripted([new ApiError("rate_limited", 429), null]);
    const sleep = async (_ms: number) => {
      loop.stop();
    };
    loop = new GameLoop(player, 4, 0, events, sleep);
    await loop.start();
    expect(player.calls).toBe(1);
    expect(log.errors).toEqual([]);
    expect(log.retries).toEqual([1]);
    expect(loop.running).toBe(false);
    expect(log.steps).toEqual([]);
  });

  it("stop requested while a failing pick is in flight ends the loop silently", async () => {
    const { log, events } = recorder();
    let loop!: GameLoop;
    let calls = 0;
    const player: Player = {
      name: "jev-sim",
      usesJev: true,
      async pick() {
        calls++;
        loop.stop();
        throw new ApiError("rate_limited", 429);
      },
    };
    loop = new GameLoop(player, 4, 0, events, noSleep);
    await loop.start();
    expect(calls).toBe(1);
    expect(log.retries).toEqual([]);
    expect(log.errors).toEqual([]);
    expect(loop.running).toBe(false);
  });

  it("ignores a second start while running", async () => {
    const { events } = recorder();
    const loop = new GameLoop(scripted([]), 4, 0, events, noSleep);
    const first = loop.start();
    await loop.start();
    await first;
    expect(loop.game.over()).toBe(true);
  });

  it("changing delayMs mid-game changes the sleep used after the next step", async () => {
    const { events } = recorder();
    const waits: number[] = [];
    let loop!: GameLoop;
    const player: Player = {
      name: "greedy",
      usesJev: false,
      async pick(board) {
        if (loop.game.moves === 0) loop.delayMs = 0;
        return { move: legalMoves(board)[0], info: {} };
      },
    };
    loop = new GameLoop(player, 4, 500, events, async (ms) => { waits.push(ms); });
    await loop.start();
    expect(waits[0]).toBe(0);
  });
});
