import { describe, expect, it } from "vitest";
import {
  ALL_MOVES, Game, type Board, emptyBoard, emptyCells, legalMoves, maxInCorner, maxTile,
  mulberry32, slide, spawn,
} from "../src/shared/game";

const row = (r: number[]): Board => [r, [0, 0, 0, 0], [0, 0, 0, 0], [0, 0, 0, 0]];
const stuck: Board = [
  [2, 4, 2, 4],
  [4, 2, 4, 2],
  [2, 4, 2, 4],
  [4, 2, 4, 2],
];

describe("slide", () => {
  it.each([
    ["all same merges pairwise", [2, 2, 2, 2], [4, 4, 0, 0], 8],
    ["merged tile does not merge again", [2, 2, 4, 0], [4, 4, 0, 0], 4],
    ["merge across a gap", [4, 0, 4, 8], [8, 8, 0, 0], 8],
    ["three same merges the leading pair", [2, 2, 2, 0], [4, 2, 0, 0], 4],
    ["nothing to merge", [2, 4, 2, 4], [2, 4, 2, 4], 0],
  ])("%s", (_name, input, want, gained) => {
    const res = slide(row(input as number[]), "left");
    expect(res.board[0]).toEqual(want);
    expect(res.gained).toBe(gained);
  });

  it("handles all four directions and counts merges", () => {
    const b: Board = [
      [2, 0, 0, 2],
      [0, 0, 0, 0],
      [0, 0, 0, 0],
      [2, 0, 0, 2],
    ];
    const z = [0, 0, 0, 0];
    const want: Record<string, Board> = {
      left: [[4, 0, 0, 0], z, z, [4, 0, 0, 0]],
      right: [[0, 0, 0, 4], z, z, [0, 0, 0, 4]],
      up: [[4, 0, 0, 4], z, z, z],
      down: [z, z, z, [4, 0, 0, 4]],
    };
    for (const m of ALL_MOVES) {
      const res = slide(b, m);
      expect(res.board).toEqual(want[m]);
      expect(res.gained).toBe(8);
      expect(res.merges).toBe(2);
      expect(res.moved).toBe(true);
    }
  });

  it("reports moved=false and leaves the input untouched", () => {
    const b = row([2, 4, 2, 4]);
    const copy = JSON.parse(JSON.stringify(b));
    for (const m of ["left", "right", "up"] as const) {
      expect(slide(b, m).moved).toBe(false);
    }
    slide(b, "down");
    expect(b).toEqual(copy);
  });
});

describe("board helpers", () => {
  it("legalMoves follows ALL_MOVES order", () => {
    expect(legalMoves(row([2, 4, 2, 4]))).toEqual(["down"]);
    expect(legalMoves(row([2, 0, 0, 0]))).toEqual(["down", "right"]);
    expect(legalMoves(stuck)).toEqual([]);
  });

  it("features", () => {
    const b: Board = [
      [64, 2, 0, 0],
      [0, 0, 0, 0],
      [0, 8, 0, 0],
      [0, 0, 0, 0],
    ];
    expect(emptyCells(b)).toBe(13);
    expect(maxTile(b)).toBe(64);
    expect(maxInCorner(b)).toBe(true);
    b[0][0] = 0;
    b[1][1] = 64;
    expect(maxInCorner(b)).toBe(false);
    expect(maxInCorner(emptyBoard())).toBe(false);
  });
});

describe("spawn and Game", () => {
  it("mulberry32 is deterministic and in [0, 1)", () => {
    const a = mulberry32(7);
    const b = mulberry32(7);
    for (let i = 0; i < 100; i++) {
      const v = a();
      expect(v).toBe(b());
      expect(v).toBeGreaterThanOrEqual(0);
      expect(v).toBeLessThan(1);
    }
  });

  it("spawn fills one empty cell with 2 or 4 and leaves a full board alone", () => {
    expect(spawn(stuck, mulberry32(1))).toEqual(stuck);
    const oneHole = stuck.map((r) => [...r]);
    oneHole[2][1] = 0;
    const got = spawn(oneHole, mulberry32(1));
    expect([2, 4]).toContain(got[2][1]);
    expect(oneHole[2][1]).toBe(0);
  });

  it("a new game has two tiles", () => {
    const g = new Game(1);
    expect(emptyCells(g.board)).toBe(14);
    expect(g.score).toBe(0);
    expect(g.moves).toBe(0);
  });

  it("same seed and same moves give the same boards", () => {
    const a = new Game(7);
    const b = new Game(7);
    for (let i = 0; i < 50 && !a.over(); i++) {
      const m = legalMoves(a.board)[0];
      a.step(m);
      b.step(m);
      expect(a.board).toEqual(b.board);
      expect(a.score).toBe(b.score);
    }
    expect(a.moves).toBeGreaterThan(0);
  });

  it("step rejects an illegal move without changing the game", () => {
    const g = new Game(1);
    g.board = row([2, 4, 2, 4]);
    expect(() => g.step("left")).toThrow("illegal move");
    expect(g.board).toEqual(row([2, 4, 2, 4]));
    expect(g.moves).toBe(0);
  });

  it("step adds the score and spawns a tile", () => {
    const g = new Game(1);
    g.board = row([2, 2, 0, 0]);
    g.step("left");
    expect(g.score).toBe(4);
    expect(g.moves).toBe(1);
    expect(emptyCells(g.board)).toBe(14);
  });

  it("over", () => {
    const g = new Game(1);
    expect(g.over()).toBe(false);
    g.board = stuck;
    expect(g.over()).toBe(true);
  });
});
