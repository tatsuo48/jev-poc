import { type Env, dailyLimit } from "./env";
import { handleApi } from "./handler";
import { askJev } from "./jev";

export { Budget } from "./budget";

export default {
  async fetch(request, env): Promise<Response> {
    const budget = env.BUDGET.getByName("global");
    const response = await handleApi(request, {
      askJev: (prompt, legal) =>
        askJev((input, init) => fetch(input, init), env.JEV_ENDPOINT, env.TYPESAFE_API_KEY, prompt, legal),
      takeBudget: () => budget.take(),
      peekBudget: () => budget.peek(),
      allowRequest: async (ip) => (env.MOVE_LIMITER ? (await env.MOVE_LIMITER.limit({ key: ip })).success : true),
      dailyLimit: dailyLimit(env.DAILY_LIMIT),
      hasApiKey: Boolean(env.TYPESAFE_API_KEY),
      log: (message) => console.error(message),
    });
    return response ?? env.ASSETS.fetch(request);
  },
} satisfies ExportedHandler<Env>;
