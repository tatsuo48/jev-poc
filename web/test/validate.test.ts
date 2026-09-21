import { describe, expect, it } from "vitest";
import { parseMoveRequest } from "../src/worker/validate";

const board = [
  [2, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
  [0, 0, 0, 0],
];

describe("parseMoveRequest", () => {
  it("accepts a valid request and computes the legal moves", () => {
    expect(parseMoveRequest({ player: "jev-sim", board })).toEqual({
      player: "jev-sim",
      board,
      legal: ["down", "right"],
    });
  });

  it.each([
    ["not an object", "x"],
    ["null", null],
    ["unknown player", { player: "expectimax", board }],
    ["missing board", { player: "jev-raw" }],
    ["three rows", { player: "jev-raw", board: board.slice(0, 3) }],
    ["a short row", { player: "jev-raw", board: [[2, 0, 0], ...board.slice(1)] }],
    ["a row that is not an array", { player: "jev-raw", board: ["2000", ...board.slice(1)] }],
    ["not a power of two", { player: "jev-raw", board: [[3, 0, 0, 0], ...board.slice(1)] }],
    ["one", { player: "jev-raw", board: [[1, 0, 0, 0], ...board.slice(1)] }],
    ["negative", { player: "jev-raw", board: [[-2, 0, 0, 0], ...board.slice(1)] }],
    ["fraction", { player: "jev-raw", board: [[2.5, 0, 0, 0], ...board.slice(1)] }],
    ["string cell", { player: "jev-raw", board: [["2", 0, 0, 0], ...board.slice(1)] }],
    ["too large", { player: "jev-raw", board: [[262144, 0, 0, 0], ...board.slice(1)] }],
    ["only one legal move", { player: "jev-raw", board: [[2, 4, 2, 4], ...board.slice(1)] }],
    ["empty board", { player: "jev-raw", board: board.map(() => [0, 0, 0, 0]) }],
  ])("rejects %s", (_name, body) => {
    expect(parseMoveRequest(body)).toBeNull();
  });
});
