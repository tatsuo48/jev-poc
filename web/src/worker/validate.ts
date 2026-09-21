import { type Board, type Move, legalMoves } from "../shared/game";
import { JEV_PLAYERS, type JevPlayer } from "../shared/prompt";

export interface MoveRequest {
  player: JevPlayer;
  board: Board;
  legal: Move[];
}

const MAX_TILE = 131072;

function isTile(v: unknown): v is number {
  return typeof v === "number" && Number.isInteger(v) && (v === 0 || (v >= 2 && v <= MAX_TILE && (v & (v - 1)) === 0));
}

/**
 * Accepts only a player name and a well-formed board. The prompt is built from
 * these on the server, so the endpoint cannot be used to send arbitrary text to jev.
 */
export function parseMoveRequest(body: unknown): MoveRequest | null {
  if (typeof body !== "object" || body === null) return null;
  const { player, board } = body as { player?: unknown; board?: unknown };
  if (!JEV_PLAYERS.includes(player as JevPlayer)) return null;
  if (!Array.isArray(board) || board.length !== 4) return null;
  for (const row of board) {
    if (!Array.isArray(row) || row.length !== 4 || !row.every(isTile)) return null;
  }
  const legal = legalMoves(board as Board);
  if (legal.length < 2) return null;
  return { player: player as JevPlayer, board: board as Board, legal };
}
