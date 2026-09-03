import { describe, expect, it } from "vitest";
import { serviceImageFor } from "./service-imagery";

describe("serviceImageFor", () => {
  it("matches known care themes to relevant supplied imagery", () => {
    expect(serviceImageFor("Nursing consultation")).toContain("037.webp");
    expect(serviceImageFor("Mindfulness and stress support")).toContain("lavender");
    expect(serviceImageFor("Nutrition coaching")).toContain("meal");
    expect(serviceImageFor("Massage and bodywork")).toContain("swan");
    expect(serviceImageFor("Introductory wellness conversation")).toContain("010.webp");
  });

  it("gives custom services a stable image fallback", () => {
    expect(serviceImageFor("A custom care session", 3)).toMatch(/^\/images\//);
    expect(serviceImageFor("A custom care session", 3)).toBe(
      serviceImageFor("A custom care session", 3),
    );
  });
});
