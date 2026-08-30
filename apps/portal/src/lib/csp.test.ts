import { describe, expect, it } from "vitest";
import nextConfig from "../../next.config";

/**
 * The customer app's Content Security Policy (PROD-06).
 *
 * A CSP is one string, and every way of getting it wrong looks the same
 * from the outside: the app works. A missing `object-src` works. A
 * `connect-src` that lost its bound and now reads `*` works. These pin the
 * directives whose absence is invisible until someone is exploiting it.
 */

async function policy(): Promise<string> {
  const groups = await nextConfig.headers!();
  const csp = groups
    .flatMap((group) => group.headers)
    .find((header) => header.key === "Content-Security-Policy");
  expect(csp, "no Content-Security-Policy header is configured").toBeDefined();
  return csp!.value;
}

function directive(csp: string, name: string): string {
  const found = csp
    .split(";")
    .map((part) => part.trim())
    .find((part) => part === name || part.startsWith(`${name} `));
  expect(found, `the policy has no ${name} directive`).toBeDefined();
  return found!;
}

describe("customer app CSP", () => {
  it("closes the directives whose absence is silent", async () => {
    const csp = await policy();

    // Flash and friends: nothing on this site needs a plugin, and object-src
    // is not covered by default-src in every browser.
    expect(directive(csp, "object-src")).toBe("object-src 'none'");
    // An injected <base> re-points every relative URL on the page.
    expect(directive(csp, "base-uri")).toBe("base-uri 'self'");
    // Without this, an injected form action posts a client's intake answers
    // to somebody else's server.
    expect(directive(csp, "form-action")).toBe("form-action 'self'");
    // Clickjacking. X-Frame-Options says the same thing to older browsers.
    expect(directive(csp, "frame-ancestors")).toBe("frame-ancestors 'none'");
  });

  it("bounds where a script may send anything", async () => {
    const csp = await policy();
    const connect = directive(csp, "connect-src");

    // This is the directive that turns an exfiltration attempt into a
    // blocked request, so a wildcard in it is the whole policy failing
    // quietly.
    expect(connect).not.toMatch(/[\s']\*/);
    expect(connect).toContain("'self'");
    // The API origin and its WebSocket spelling — a wss: URL is not covered
    // by the https: entry.
    expect(connect).toMatch(/https?:\/\/\S+/);
    expect(connect).toMatch(/wss?:\/\/\S+/);
  });

  it("never allows eval in a production build", async () => {
    // React uses eval in development to rebuild server error stacks. A
    // production bundle does not, and shipping the allowance anyway hands
    // an injected string an interpreter.
    expect(process.env.NODE_ENV).not.toBe("development");
    expect(await policy()).not.toContain("unsafe-eval");
  });

  it("ships alongside the framework-independent headers", async () => {
    const groups = await nextConfig.headers!();
    const keys = groups.flatMap((group) => group.headers).map((header) => header.key);

    // CSP replaces none of these: they are what an older browser, or one
    // that fails to parse the policy, still honours.
    for (const key of [
      "X-Content-Type-Options",
      "X-Frame-Options",
      "Referrer-Policy",
      "Permissions-Policy",
      "Cross-Origin-Opener-Policy",
    ]) {
      expect(keys).toContain(key);
    }
  });
});
