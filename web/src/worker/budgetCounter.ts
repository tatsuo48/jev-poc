export interface BudgetState {
  day: string; // UTC date, YYYY-MM-DD
  used: number;
}

export interface BudgetStore {
  load(): Promise<BudgetState | undefined>;
  save(state: BudgetState): Promise<void>;
}

/** Counts jev calls per UTC day. A single record is kept, so yesterday's count disappears on the first take of a new day. */
export class BudgetCounter {
  constructor(
    private readonly store: BudgetStore,
    private readonly limit: number,
    private readonly now: () => Date = () => new Date(),
  ) {}

  private async today(): Promise<BudgetState> {
    const day = this.now().toISOString().slice(0, 10);
    const state = await this.store.load();
    return state && state.day === day ? state : { day, used: 0 };
  }

  /** Reserves one call. A reserved call is never given back, even if the upstream call fails later. */
  async take(): Promise<{ ok: boolean; remaining: number }> {
    const state = await this.today();
    if (state.used >= this.limit) return { ok: false, remaining: 0 };
    state.used++;
    await this.store.save(state);
    return { ok: true, remaining: this.limit - state.used };
  }

  async peek(): Promise<number> {
    const state = await this.today();
    return Math.max(0, this.limit - state.used);
  }
}
