import { describe, expect, it } from "vitest";
import { membershipRoutePaths } from "./membershipRoutes";

describe("membershipRoutePaths", () => {
  it("routes payment gateway returns to the existing membership page", () => {
    expect(membershipRoutePaths).toEqual(["/membership", "/membership/complete"]);
  });
});
