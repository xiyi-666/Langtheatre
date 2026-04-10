import { describe, expect, it } from "vitest";
import { useAppStore } from "./store";

describe("useAppStore", () => {
  it("should set user", () => {
    useAppStore.getState().setUser({
      id: "1",
      email: "demo@linguaquest.app",
      totalXP: 10
    });
    expect(useAppStore.getState().user?.email).toBe("demo@linguaquest.app");
  });
});
