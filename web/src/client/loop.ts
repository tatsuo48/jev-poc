import { type Board, Game, type Move, maxTile } from "../shared/game";
import { ApiError, type PickInfo, type Player } from "./players";

export interface StepView {
  board: Board;
  score: number;
  moves: number;
  move: Move;
  latencyMs: number;
  info: PickInfo;
}

export interface LoopEvents {
  onStep(step: StepView): void;
  onRetry(attempt: number): void;
  onFinish(result: { score: number; moves: number; maxTile: number }): void;
  onError(code: string): void;
}

const MAX_RETRIES = 3;
const RETRYABLE = new Set(["rate_limited", "upstream_error", "network_error", "service_unavailable"]);

const realSleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

/**
 * Drives one game. A failed pick is retried a few times and then reported;
 * it is never replaced by another move. After an error, start() resumes the same game.
 */
export class GameLoop {
  readonly game: Game;
  running = false;
  private stopped = false;

  constructor(
    private readonly player: Player,
    seed: number,
    public delayMs: number,
    private readonly events: LoopEvents,
    private readonly sleep: (ms: number) => Promise<void> = realSleep,
  ) {
    this.game = new Game(seed);
  }

  stop(): void {
    this.stopped = true;
  }

  async start(): Promise<void> {
    if (this.running) return;
    this.running = true;
    this.stopped = false;
    try {
      while (!this.game.over() && !this.stopped) {
        const started = performance.now();
        const picked = await this.pickWithRetry();
        if (!picked || this.stopped) return;
        this.game.step(picked.move);
        this.events.onStep({
          board: this.game.board,
          score: this.game.score,
          moves: this.game.moves,
          move: picked.move,
          latencyMs: performance.now() - started,
          info: picked.info,
        });
        await this.sleep(this.delayMs); // also yields to the browser so it can paint
      }
      if (this.game.over()) {
        this.events.onFinish({ score: this.game.score, moves: this.game.moves, maxTile: maxTile(this.game.board) });
      }
    } finally {
      this.running = false;
    }
  }

  private async pickWithRetry(): Promise<{ move: Move; info: PickInfo } | null> {
    for (let attempt = 0; ; attempt++) {
      try {
        return await this.player.pick(this.game.board);
      } catch (err) {
        const code = err instanceof ApiError ? err.code : "unexpected_error";
        if (!RETRYABLE.has(code) || attempt === MAX_RETRIES || this.stopped) {
          this.events.onError(code);
          return null;
        }
        this.events.onRetry(attempt + 1);
        await this.sleep(1000 * 2 ** attempt);
      }
    }
  }
}
