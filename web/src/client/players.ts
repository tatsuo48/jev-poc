import { type Board, type Move, cloneBoard, emptyCells, legalMoves, maxInCorner, maxTile, mulberry32, slide } from "../shared/game";
import type { JevPlayer } from "../shared/prompt";

export type PlayerName = "jev-sim" | "jev-raw" | "expectimax" | "greedy" | "random";

export interface PickInfo {
  probabilities?: Partial<Record<Move, number>>;
  confidence?: number;
  remaining?: number;
}

export interface Player {
  name: PlayerName;
  usesJev: boolean;
  pick(board: Board): Promise<{ move: Move; info: PickInfo }>;
}

/** A failed /api/move call. code is the API's error code, or network_error / upstream_error. */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
  ) {
    super(code);
  }
}

const EXPECTIMAX_DEPTH = 3;

/** Maps an error response's HTTP status to a code when the body carries no `error` field (Cloudflare's own error pages, e.g. HTML 429/5xx). */
function statusErrorCode(status: number): string {
  if (status === 429) return "rate_limited";
  if (status >= 500) return "service_unavailable";
  return "upstream_error";
}

function requireLegal(board: Board): Move[] {
  const legal = legalMoves(board);
  if (legal.length === 0) throw new Error("no legal moves");
  return legal;
}

/**
 * Derives the random player's RNG seed from the game seed so its move stream never lines up
 * with the game's own tile-spawn RNG (both are mulberry32 seeded from the same input).
 */
export function playerSeed(seed: number): number {
  return (seed ^ 0x9e3779b9) >>> 0;
}

function randomPlayer(seed: number): Player {
  const rand = mulberry32(playerSeed(seed));
  return {
    name: "random",
    usesJev: false,
    async pick(board) {
      const legal = requireLegal(board);
      return { move: legal[Math.floor(rand() * legal.length)], info: {} };
    },
  };
}

function greedyPlayer(): Player {
  return {
    name: "greedy",
    usesJev: false,
    async pick(board) {
      let best = requireLegal(board)[0];
      let bestGain = -1;
      let bestEmpty = -1;
      for (const m of legalMoves(board)) {
        const res = slide(board, m);
        const empty = emptyCells(res.board);
        if (res.gained > bestGain || (res.gained === bestGain && empty > bestEmpty)) {
          [best, bestGain, bestEmpty] = [m, res.gained, empty];
        }
      }
      return { move: best, info: {} };
    },
  };
}

const log2 = (v: number) => (v === 0 ? 0 : Math.log2(v));

/** 0 when every row and column is sorted in one direction; more negative the more the lines zigzag. */
export function monotonicity(b: Board): number {
  let total = 0;
  for (let i = 0; i < 4; i++) {
    let rowInc = 0, rowDec = 0, colInc = 0, colDec = 0;
    for (let k = 0; k < 3; k++) {
      const dr = log2(b[i][k + 1]) - log2(b[i][k]);
      if (dr > 0) rowInc += dr; else rowDec -= dr;
      const dc = log2(b[k + 1][i]) - log2(b[k][i]);
      if (dc > 0) colInc += dc; else colDec -= dc;
    }
    total -= Math.min(rowInc, rowDec) + Math.min(colInc, colDec);
  }
  return total;
}

function heuristic(b: Board): number {
  let h = 30 * emptyCells(b) + 10 * monotonicity(b);
  if (maxInCorner(b)) h += 20 * log2(maxTile(b));
  return h;
}

/** Looks `depth` player-moves ahead, averaging over tile spawns. Same evaluation as the Go CLI. */
function expectimaxPlayer(depth: number): Player {
  const best = (b: Board, d: number): number => {
    const legal = legalMoves(b);
    if (legal.length === 0) return -1e6;
    return Math.max(...legal.map((m) => chance(slide(b, m).board, d - 1)));
  };
  const chance = (b: Board, d: number): number => {
    if (d === 0) return heuristic(b);
    let sum = 0;
    let cells = 0;
    for (let r = 0; r < 4; r++) {
      for (let c = 0; c < 4; c++) {
        if (b[r][c] !== 0) continue;
        cells++;
        const next = cloneBoard(b);
        next[r][c] = 2;
        sum += 0.9 * best(next, d);
        next[r][c] = 4;
        sum += 0.1 * best(next, d);
      }
    }
    return cells === 0 ? best(b, d) : sum / cells;
  };
  return {
    name: "expectimax",
    usesJev: false,
    async pick(board) {
      let bestMove = requireLegal(board)[0];
      let bestValue = -Infinity;
      for (const m of legalMoves(board)) {
        const v = chance(slide(board, m).board, depth - 1);
        if (v > bestValue) [bestMove, bestValue] = [m, v];
      }
      return { move: bestMove, info: {} };
    },
  };
}

function jevPlayer(name: JevPlayer, fetcher: typeof fetch): Player {
  return {
    name,
    usesJev: true,
    async pick(board) {
      const legal = requireLegal(board);
      if (legal.length === 1) return { move: legal[0], info: {} };

      let res: Response;
      try {
        res = await fetcher("/api/move", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ player: name, board }),
        });
      } catch {
        throw new ApiError("network_error", 0);
      }
      const data = (await res.json().catch(() => ({}))) as {
        error?: string; move?: Move; probabilities?: PickInfo["probabilities"]; confidence?: number; remaining?: number;
      };
      if (!res.ok) throw new ApiError(data.error ?? statusErrorCode(res.status), res.status);
      if (!data.move || !legal.includes(data.move)) throw new ApiError("upstream_error", res.status);
      return {
        move: data.move,
        info: { probabilities: data.probabilities, confidence: data.confidence, remaining: data.remaining },
      };
    },
  };
}

/** Players may hold per-game state (the random player's RNG), so create one per game. */
export function createPlayer(
  name: PlayerName,
  seed: number,
  fetcher: typeof fetch = (input, init) => fetch(input, init),
  expectimaxDepth = EXPECTIMAX_DEPTH,
): Player {
  switch (name) {
    case "random":
      return randomPlayer(seed);
    case "greedy":
      return greedyPlayer();
    case "expectimax":
      return expectimaxPlayer(expectimaxDepth);
    default:
      return jevPlayer(name, fetcher);
  }
}

export const PLAYER_NAMES: readonly PlayerName[] = ["jev-sim", "jev-raw", "expectimax", "greedy", "random"];
