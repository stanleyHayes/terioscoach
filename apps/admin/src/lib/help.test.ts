import { describe, expect, it } from "vitest";
import { ADMIN_HELP_TOPICS, adminHelpForPath } from "./help";

describe("admin help", () => {
  it("covers every main practice workspace", () => {
    expect(ADMIN_HELP_TOPICS.length).toBeGreaterThanOrEqual(16);
    expect(ADMIN_HELP_TOPICS.every((topic) => topic.goal && topic.steps.length >= 3)).toBe(true);
  });

  it("prefers the most specific nested-page guide", () => {
    expect(adminHelpForPath("/content/posts/new").title).toBe("Blog editor");
    expect(adminHelpForPath("/clients/client-1").title).toBe("Clients");
  });
});
