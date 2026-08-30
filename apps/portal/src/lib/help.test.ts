import { describe, expect, it } from "vitest";
import { PORTAL_HELP_TOPICS, portalHelpForPath } from "./help";

describe("portal help", () => {
  it("gives every client task a goal and ordered instructions", () => {
    expect(PORTAL_HELP_TOPICS.length).toBeGreaterThanOrEqual(10);
    expect(PORTAL_HELP_TOPICS.every((topic) => topic.goal && topic.steps.length >= 3)).toBe(true);
  });

  it("recognizes a dynamic consultation room", () => {
    expect(portalHelpForPath("/portal/sessions/booking-1/room").title).toBe("Video consultation");
    expect(portalHelpForPath("/portal/sessions").title).toBe("Consultations");
  });
});
