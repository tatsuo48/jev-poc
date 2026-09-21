import { describe, expect, it } from "vitest";
import type { Board } from "../src/shared/game";
import { INSTRUCTIONS, buildPrompt } from "../src/shared/prompt";

const cornerTile: Board = [
  [2, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
];

describe("buildPrompt", () => {
  it("uses the same instructions as the Go CLI", () => {
    expect(INSTRUCTIONS).toBe(
      "You are playing the game 2048 on a 4x4 board, given as rows from top to bottom (0 = empty cell). " +
        "Choose the move most likely to lead to the highest final score in the long run. " +
        "Good play keeps the largest tile in a corner, keeps many cells empty, " +
        "and keeps rows and columns sorted in one direction.",
    );
  });

  it("jev-raw sends the board only and describes the legal directions", () => {
    const p = buildPrompt("jev-raw", cornerTile, ["down", "right"]);
    expect(p.state).toEqual({ board: cornerTile });
    expect(p.criteria).toEqual({
      down: "Slide all tiles toward the bottom row.",
      right: "Slide all tiles toward the rightmost column.",
    });
  });

  it("jev-sim describes the outcome of every legal move", () => {
    const b: Board = [
      [2, 2, 0, 0],
      [0, 0, 0, 0],
      [0, 0, 0, 0],
      [0, 0, 0, 0],
    ];
    const p = buildPrompt("jev-sim", b, ["down", "left", "right"]);
    const state = p.state as { board: Board; candidates: Record<string, Record<string, unknown>> };
    expect(state.board).toEqual(b);
    expect(Object.keys(state.candidates).sort()).toEqual(["down", "left", "right"]);
    expect(state.candidates.left).toEqual({
      board: [[4, 0, 0, 0], [0, 0, 0, 0], [0, 0, 0, 0], [0, 0, 0, 0]],
      gained: 4,
      empty_cells: 15,
      merges: 1,
      max_tile: 4,
      max_in_corner: true,
    });
    expect(state.candidates.down).toMatchObject({ gained: 0, merges: 0, empty_cells: 14 });
    expect(p.criteria.left).toBe(
      "Slide all tiles toward the leftmost column. The resulting board and its features are in state.candidates.left.",
    );
    expect(Object.keys(p.criteria).sort()).toEqual(["down", "left", "right"]);
  });
});
