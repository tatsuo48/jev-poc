import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // The classic-player tests play full games with the depth-3 expectimax AI,
    // which is slow in unoptimized JS and exceeds vitest's default 5s timeout.
    testTimeout: 60000,
  },
});
