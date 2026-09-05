import type { User } from "./types";

export const DEMO_USERNAME = "lingua_demo_0903";
const DEMO_DIFFICULTY_KEY = "linguaquest.demo.difficulty";
const DEMO_DIFFICULTIES = [5.5, 6.5, 7.5] as const;
export const DEMO_GENERATION_MIN_MS = 1200;

export function isDemoUser(user?: Pick<User, "username">): boolean {
  return user?.username.toLowerCase() === DEMO_USERNAME;
}

export function getDemoSessionDifficulty(user?: Pick<User, "username">): number | undefined {
  if (!isDemoUser(user)) return undefined;
  const stored = Number.parseFloat(sessionStorage.getItem(DEMO_DIFFICULTY_KEY) ?? "");
  if (DEMO_DIFFICULTIES.includes(stored as (typeof DEMO_DIFFICULTIES)[number])) return stored;
  const random = new Uint32Array(1);
  crypto.getRandomValues(random);
  const assigned = DEMO_DIFFICULTIES[random[0] % DEMO_DIFFICULTIES.length];
  sessionStorage.setItem(DEMO_DIFFICULTY_KEY, assigned.toFixed(1));
  return assigned;
}

export function waitForDemoGeneration(startedAt: number, minimumMs = DEMO_GENERATION_MIN_MS): Promise<void> {
  const remaining = Math.max(0, minimumMs - (Date.now() - startedAt));
  if (remaining === 0) return Promise.resolve();
  return new Promise((resolve) => window.setTimeout(resolve, remaining));
}
