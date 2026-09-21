// Builds what is sent to jev. The wording mirrors internal/player/jev.go of the Go CLI.
import { type Board, type Move, emptyCells, maxInCorner, maxTile, slide } from "./game";

export type JevPlayer = "jev-raw" | "jev-sim";
export const JEV_PLAYERS: readonly JevPlayer[] = ["jev-raw", "jev-sim"];

export const INSTRUCTIONS =
  "You are playing the game 2048 on a 4x4 board, given as rows from top to bottom (0 = empty cell). " +
  "Choose the move most likely to lead to the highest final score in the long run. " +
  "Good play keeps the largest tile in a corner, keeps many cells empty, " +
  "and keeps rows and columns sorted in one direction.";

const DIRECTION_HELP: Record<Move, string> = {
  up: "Slide all tiles toward the top row.",
  down: "Slide all tiles toward the bottom row.",
  left: "Slide all tiles toward the leftmost column.",
  right: "Slide all tiles toward the rightmost column.",
};

export interface Prompt {
  state: unknown;
  criteria: Record<string, string>;
}

/** Only the legal moves are offered, so jev cannot answer with an illegal one. */
export function buildPrompt(player: JevPlayer, board: Board, legal: Move[]): Prompt {
  const criteria: Record<string, string> = {};
  if (player === "jev-raw") {
    for (const m of legal) criteria[m] = DIRECTION_HELP[m];
    return { state: { board }, criteria };
  }

  const candidates: Record<string, unknown> = {};
  for (const m of legal) {
    const res = slide(board, m);
    candidates[m] = {
      board: res.board,
      gained: res.gained,
      empty_cells: emptyCells(res.board),
      merges: res.merges,
      max_tile: maxTile(res.board),
      max_in_corner: maxInCorner(res.board),
    };
    criteria[m] = `${DIRECTION_HELP[m]} The resulting board and its features are in state.candidates.${m}.`;
  }
  return { state: { board, candidates }, criteria };
}
