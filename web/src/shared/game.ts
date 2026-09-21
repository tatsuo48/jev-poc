// Rules of 2048 as pure functions over a 4x4 board. Mirrors internal/game of the Go CLI.

export type Move = "up" | "down" | "left" | "right";

/** Fixed order used for iteration and tie-breaking. */
export const ALL_MOVES: readonly Move[] = ["up", "down", "left", "right"];

/** Tile values; 0 is an empty cell. */
export type Board = number[][];

export function emptyBoard(): Board {
  return Array.from({ length: 4 }, () => [0, 0, 0, 0]);
}

export function cloneBoard(b: Board): Board {
  return b.map((r) => [...r]);
}

export function boardsEqual(a: Board, b: Board): boolean {
  for (let r = 0; r < 4; r++) {
    for (let c = 0; c < 4; c++) {
      if (a[r][c] !== b[r][c]) return false;
    }
  }
  return true;
}

/** Coordinates of the k-th cell of line i, counted from the edge the tiles travel toward. */
function cell(move: Move, i: number, k: number): [number, number] {
  switch (move) {
    case "left":
      return [i, k];
    case "right":
      return [i, 3 - k];
    case "up":
      return [k, i];
    default:
      return [3 - k, i];
  }
}

/** Packs a line toward index 0. A tile created by a merge never merges again in the same move. */
function slideLine(line: number[]): { out: number[]; gained: number; merges: number } {
  const out = [0, 0, 0, 0];
  let idx = 0;
  let last = 0; // value of the last placed tile that is still mergeable
  let gained = 0;
  let merges = 0;
  for (const v of line) {
    if (v === 0) continue;
    if (v === last) {
      out[idx - 1] = v * 2;
      gained += v * 2;
      merges++;
      last = 0;
      continue;
    }
    out[idx++] = v;
    last = v;
  }
  return { out, gained, merges };
}

export interface SlideResult {
  board: Board;
  gained: number;
  merges: number;
  moved: boolean;
}

/** Applies a move without spawning a tile. The input board is not modified. */
export function slide(b: Board, move: Move): SlideResult {
  const board = emptyBoard();
  let gained = 0;
  let merges = 0;
  for (let i = 0; i < 4; i++) {
    const line = [0, 1, 2, 3].map((k) => {
      const [r, c] = cell(move, i, k);
      return b[r][c];
    });
    const res = slideLine(line);
    gained += res.gained;
    merges += res.merges;
    for (let k = 0; k < 4; k++) {
      const [r, c] = cell(move, i, k);
      board[r][c] = res.out[k];
    }
  }
  return { board, gained, merges, moved: !boardsEqual(board, b) };
}

/** Moves that change the board, in ALL_MOVES order. */
export function legalMoves(b: Board): Move[] {
  return ALL_MOVES.filter((m) => slide(b, m).moved);
}

export function emptyCells(b: Board): number {
  return b.flat().filter((v) => v === 0).length;
}

export function maxTile(b: Board): number {
  return Math.max(...b.flat());
}

export function maxInCorner(b: Board): boolean {
  const max = maxTile(b);
  return max > 0 && [b[0][0], b[0][3], b[3][0], b[3][3]].includes(max);
}

/** Small seeded PRNG. The Go CLI uses a different generator, so seeds do not match across the two. */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Returns a copy of b with a 2 (90%) or a 4 (10%) on a random empty cell. */
export function spawn(b: Board, rand: () => number): Board {
  const empty: [number, number][] = [];
  b.forEach((row, r) => row.forEach((v, c) => v === 0 && empty.push([r, c])));
  if (empty.length === 0) return b;
  const [r, c] = empty[Math.floor(rand() * empty.length)];
  const next = cloneBoard(b);
  next[r][c] = rand() < 0.1 ? 4 : 2;
  return next;
}

/** One playthrough. The same seed and the same moves always give the same boards. */
export class Game {
  board: Board;
  score = 0;
  moves = 0;
  private readonly rand: () => number;

  constructor(seed: number) {
    this.rand = mulberry32(seed);
    this.board = spawn(spawn(emptyBoard(), this.rand), this.rand);
  }

  step(move: Move): void {
    const res = slide(this.board, move);
    if (!res.moved) throw new Error("illegal move");
    this.board = spawn(res.board, this.rand);
    this.score += res.gained;
    this.moves++;
  }

  over(): boolean {
    return legalMoves(this.board).length === 0;
  }
}
