import { DurableObject } from "cloudflare:workers";
import { BudgetCounter, type BudgetState } from "./budgetCounter";
import { type Env, dailyLimit } from "./env";

/** One instance (named "global") counts jev calls for all visitors. */
export class Budget extends DurableObject<Env> {
  private readonly counter: BudgetCounter;

  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
    this.counter = new BudgetCounter(
      {
        load: () => ctx.storage.get<BudgetState>("state"),
        save: (state) => ctx.storage.put("state", state),
      },
      dailyLimit(env.DAILY_LIMIT),
    );
  }

  take() {
    return this.counter.take();
  }

  peek() {
    return this.counter.peek();
  }
}
