import { ALL_MOVES, type Board, type Move } from "../shared/game";
import type { PickInfo } from "./players";

const MOVE_LABEL: Record<Move, string> = { up: "↑ up", down: "↓ down", left: "← left", right: "→ right" };

export function renderBoard(el: HTMLElement, board: Board): void {
  el.replaceChildren(
    ...board.flat().map((v) => {
      const cell = document.createElement("div");
      cell.className = "tile";
      cell.dataset.value = String(Math.min(v, 4096));
      cell.textContent = v === 0 ? "" : String(v);
      return cell;
    }),
  );
}

/** Shows one bar per offered move. Hidden for classic players and for turns where jev was not asked. */
export function renderBars(el: HTMLElement, info: PickInfo, chosen?: Move): void {
  const probabilities = info.probabilities ?? {};
  const offered = ALL_MOVES.filter((m) => typeof probabilities[m] === "number");
  el.hidden = offered.length === 0;
  if (el.hidden) return;

  const rows = offered.map((m) => {
    const p = Math.min(1, Math.max(0, probabilities[m] ?? 0));
    const row = document.createElement("div");
    row.className = m === chosen ? "bar-row chosen" : "bar-row";
    const label = document.createElement("span");
    label.className = "bar-label";
    label.textContent = MOVE_LABEL[m];
    const track = document.createElement("span");
    track.className = "bar-track";
    const fill = document.createElement("span");
    fill.className = "bar-fill";
    fill.style.width = `${(p * 100).toFixed(1)}%`;
    track.append(fill);
    const value = document.createElement("span");
    value.className = "bar-value";
    value.textContent = p.toFixed(2);
    row.append(label, track, value);
    return row;
  });
  const confidenceValue = Math.min(1, Math.max(0, info.confidence ?? 0));
  const confidence = document.createElement("div");
  confidence.className = "confidence";
  confidence.textContent = `confidence ${confidenceValue.toFixed(2)}`;
  el.replaceChildren(...rows, confidence);
}

export function setText(el: HTMLElement, text: string): void {
  el.textContent = text;
}
