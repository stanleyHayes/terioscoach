import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Analytics } from "./Analytics";

const pathname = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ usePathname: pathname }));

// The real components inject scripts; what matters here is only whether
// they are rendered at all.
vi.mock("@vercel/analytics/react", () => ({
  Analytics: () => <div data-testid="vercel-analytics" />,
}));
vi.mock("@vercel/speed-insights/next", () => ({
  SpeedInsights: () => <div data-testid="speed-insights" />,
}));

describe("Analytics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // isIndexable() reads the deployment origin; production by default here.
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://terioswellness.com");
    vi.stubEnv("VERCEL_ENV", "production");
  });

  afterEach(() => vi.unstubAllEnvs());

  it("measures the public site", () => {
    pathname.mockReturnValue("/services");
    const { queryByTestId } = render(<Analytics />);

    expect(queryByTestId("vercel-analytics")).not.toBeNull();
    expect(queryByTestId("speed-insights")).not.toBeNull();
  });

  it.each(["/portal", "/portal/sessions/abc123", "/login", "/register"])(
    "does not measure %s",
    (path) => {
      pathname.mockReturnValue(path);
      const { queryByTestId } = render(<Analytics />);

      // A signed-in client's URLs carry session ids and belong to their own
      // health record. They are not a funnel to optimise.
      expect(queryByTestId("vercel-analytics")).toBeNull();
      expect(queryByTestId("speed-insights")).toBeNull();
    },
  );

  it("does not measure a preview deployment", () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://terios-web-git-branch.vercel.app");
    vi.stubEnv("VERCEL_ENV", "preview");
    pathname.mockReturnValue("/");

    const { queryByTestId } = render(<Analytics />);

    // Staging traffic in the production numbers makes them useless — the
    // same reason previews refuse indexing.
    expect(queryByTestId("vercel-analytics")).toBeNull();
  });

  it("sets no cookies, which is why there is no consent banner", () => {
    pathname.mockReturnValue("/");
    const before = document.cookie;
    render(<Analytics />);
    expect(document.cookie).toBe(before);
  });
});
