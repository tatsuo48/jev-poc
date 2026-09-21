import type { Move } from "../shared/game";
import { INSTRUCTIONS, type Prompt } from "../shared/prompt";

const MODEL = "jev-latest";
const TIMEOUT_MS = 10_000;
const QUESTION_KEY = "move";

export type HttpFetch = (input: string, init: RequestInit) => Promise<Response>;

export interface JevAnswer {
  move: Move;
  probabilities: Partial<Record<Move, number>>;
  confidence: number;
}

/** Carries only a status code and a coarse reason; never the upstream body, which could echo the key. */
export class UpstreamError extends Error {
  constructor(
    readonly status: number,
    readonly reason: "status" | "network" | "bad_answer",
  ) {
    super(`jev upstream error: ${reason} ${status}`);
  }
}

/** Calls jev exactly once. Retrying is the browser's job. */
export async function askJev(
  fetcher: HttpFetch,
  endpoint: string,
  apiKey: string,
  prompt: Prompt,
  legal: Move[],
): Promise<JevAnswer> {
  let res: Response;
  try {
    res = await fetcher(endpoint, {
      method: "POST",
      headers: { Authorization: `Bearer ${apiKey}`, "Content-Type": "application/json" },
      body: JSON.stringify({
        model: MODEL,
        state: prompt.state,
        questions: { [QUESTION_KEY]: { type: "choice", instructions: INSTRUCTIONS, criteria: prompt.criteria } },
      }),
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
  } catch {
    throw new UpstreamError(0, "network");
  }
  if (res.status !== 200) throw new UpstreamError(res.status, "status");

  let data: { answers?: Record<string, { choice?: unknown; probabilities?: unknown; confidence?: unknown }> };
  try {
    data = (await res.json()) as typeof data;
  } catch {
    throw new UpstreamError(200, "bad_answer");
  }
  const answer = data?.answers?.[QUESTION_KEY];
  const move = legal.find((m) => m === answer?.choice);
  if (!answer || !move) throw new UpstreamError(200, "bad_answer");

  const probabilities: Partial<Record<Move, number>> = {};
  const raw = (answer.probabilities ?? {}) as Record<string, unknown>;
  for (const m of legal) {
    const p = raw[m];
    if (typeof p === "number" && Number.isFinite(p)) probabilities[m] = p;
  }
  const confidence = typeof answer.confidence === "number" && Number.isFinite(answer.confidence) ? answer.confidence : 0;
  return { move, probabilities, confidence };
}
