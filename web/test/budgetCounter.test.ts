import { describe, expect, it } from "vitest";
import { BudgetCounter, type BudgetState, type BudgetStore } from "../src/worker/budgetCounter";

function memoryStore(): BudgetStore & { state?: BudgetState; saves: number } {
  const store = {
    state: undefined as BudgetState | undefined,
    saves: 0,
    load: async () => (store.state ? { ...store.state } : undefined),
    save: async (s: BudgetState) => {
      store.state = { ...s };
      store.saves++;
    },
  };
  return store;
}

describe("BudgetCounter", () => {
  it("allows exactly `limit` takes per UTC day", async () => {
    const store = memoryStore();
    const counter = new BudgetCounter(store, 3, () => new Date("2026-09-21T10:00:00Z"));
    expect(await counter.peek()).toBe(3);
    expect(await counter.take()).toEqual({ ok: true, remaining: 2 });
    expect(await counter.take()).toEqual({ ok: true, remaining: 1 });
    expect(await counter.take()).toEqual({ ok: true, remaining: 0 });
    expect(await counter.take()).toEqual({ ok: false, remaining: 0 });
    expect(await counter.peek()).toBe(0);
    expect(store.saves).toBe(3);
  });

  it("peek does not consume", async () => {
    const counter = new BudgetCounter(memoryStore(), 2, () => new Date("2026-09-21T10:00:00Z"));
    await counter.peek();
    await counter.peek();
    expect(await counter.take()).toEqual({ ok: true, remaining: 1 });
  });

  it("resets when the UTC date changes", async () => {
    const store = memoryStore();
    let now = new Date("2026-09-21T23:59:59Z");
    const counter = new BudgetCounter(store, 1, () => now);
    expect((await counter.take()).ok).toBe(true);
    expect((await counter.take()).ok).toBe(false);
    now = new Date("2026-09-22T00:00:00Z");
    expect(await counter.peek()).toBe(1);
    expect(await counter.take()).toEqual({ ok: true, remaining: 0 });
    expect(store.state).toEqual({ day: "2026-09-22", used: 1 });
  });
});
