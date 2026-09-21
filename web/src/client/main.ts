import { emptyBoard } from "../shared/game";
import { GameLoop } from "./loop";
import { PLAYER_NAMES, type PlayerName, createPlayer } from "./players";
import { renderBars, renderBoard, setText } from "./view";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

const playerSelect = $<HTMLSelectElement>("player");
const seedInput = $<HTMLInputElement>("seed");
const speedSelect = $<HTMLSelectElement>("speed");
const startButton = $<HTMLButtonElement>("start");
const boardEl = $("board");
const barsEl = $("bars");
const statsEl = $("stats");
const messageEl = $("message");
const remainingEl = $("remaining");

const ERROR_TEXT: Record<string, string> = {
  daily_budget_exhausted: "本日の jev は終了しました。古典 AI(expectimax / greedy / random)は引き続き遊べます。",
  rate_limited: "アクセスが集中しています。少し待ってから「再開する」を押してください。",
  network_error: "通信に失敗しました。「再開する」で続きから打てます。",
  upstream_error: "jev から応答を得られませんでした。「再開する」で続きから打てます。",
  service_unavailable: "サービスが一時的に利用できません。少し待ってから「再開する」を押してください。",
  internal_error: "サーバーで問題が起きました。少し待ってから「再開する」を押してください。",
};

let loop: GameLoop | null = null;
// Runs are chained so that "resume" pressed right after "stop" waits for the old run to wind down.
let lastRun: Promise<void> = Promise.resolve();

function showRemaining(remaining: number | undefined): void {
  if (typeof remaining === "number") setText(remainingEl, `本日の jev 残り: ${remaining.toLocaleString()} 手`);
}

async function refreshRemaining(): Promise<void> {
  try {
    const res = await fetch("/api/status");
    if (res.ok) showRemaining(((await res.json()) as { remaining: number }).remaining);
  } catch {
    // The counter is informational; the game works without it.
  }
}

function setRunning(running: boolean, resumable = false): void {
  startButton.textContent = running ? "■ ストップ" : resumable ? "▶ 再開する" : "▶ スタート";
  startButton.dataset.mode = running ? "stop" : resumable ? "resume" : "start";
  playerSelect.disabled = seedInput.disabled = running;
}

function newLoop(): GameLoop {
  const name = playerSelect.value as PlayerName;
  const seed = Number.parseInt(seedInput.value, 10) || 0;
  const player = createPlayer(name, seed);
  const created = new GameLoop(player, seed, Number(speedSelect.value), {
    onStep(step) {
      renderBoard(boardEl, step.board);
      renderBars(barsEl, step.info, step.move);
      setText(statsEl, `score ${step.score.toLocaleString()}　moves ${step.moves}　last ${step.move}　${Math.round(step.latencyMs)}ms`);
      setText(messageEl, "");
      showRemaining(step.info.remaining);
    },
    onRetry(attempt) {
      setText(messageEl, `応答待ち… 再試行 ${attempt} / 3`);
    },
    onFinish(result) {
      setText(messageEl, `ゲーム終了: score ${result.score.toLocaleString()}、${result.moves} 手、最大タイル ${result.maxTile}`);
      setRunning(false);
    },
    onError(code) {
      setText(messageEl, ERROR_TEXT[code] ?? `エラーが起きました (${code})`);
      setRunning(false, code !== "daily_budget_exhausted");
      if (code === "daily_budget_exhausted") showRemaining(0);
    },
  });
  renderBoard(boardEl, created.game.board);
  renderBars(barsEl, {});
  setText(statsEl, "score 0　moves 0");
  return created;
}

startButton.addEventListener("click", () => {
  const mode = startButton.dataset.mode;
  if (mode === "stop") {
    loop?.stop();
    setRunning(false, true);
    return;
  }
  if (mode !== "resume" || !loop) loop = newLoop();
  setText(messageEl, "");
  setRunning(true);
  const current = loop;
  lastRun = lastRun.then(() => current.start()).catch(() => {
    setText(messageEl, "表示の更新中に問題が起きました。「再開する」で続きから打てます。");
    setRunning(false, true);
  });
});

for (const name of PLAYER_NAMES) playerSelect.add(new Option(name, name));
for (const el of [playerSelect, seedInput]) {
  el.addEventListener("change", () => {
    loop = null;
    setRunning(false);
  });
}
speedSelect.addEventListener("change", () => {
  if (loop) loop.delayMs = Number(speedSelect.value);
});
renderBoard(boardEl, emptyBoard());
setRunning(false);
void refreshRemaining();
