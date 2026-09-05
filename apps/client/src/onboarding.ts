export const ONBOARDING_VERSION = "v1";

export function getOnboardingStorageKey(userId: string): string {
  return `linguaquest:onboarding:${ONBOARDING_VERSION}:${userId}`;
}

export function getOnboardingPendingKey(userId: string): string {
  return `linguaquest:onboarding:pending:${userId}`;
}

export function isOnboardingComplete(userId: string, storage: Pick<Storage, "getItem"> = localStorage): boolean {
  return storage.getItem(getOnboardingStorageKey(userId)) === "completed";
}

export function markOnboardingComplete(userId: string, storage: Pick<Storage, "setItem" | "removeItem"> = localStorage): void {
  storage.setItem(getOnboardingStorageKey(userId), "completed");
  storage.removeItem(getOnboardingPendingKey(userId));
}
