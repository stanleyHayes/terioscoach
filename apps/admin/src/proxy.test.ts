import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";
import { config, proxy } from "./proxy";

/**
 * The practice dashboard's Content Security Policy (PROD-06).
 *
 * A CSP is one string, and every way of getting it wrong looks the same
 * from the outside: the app works. These pin the parts whose absence is
 * invisible until someone is exploiting it, plus the two properties the
 * nonce scheme depends on — that it is fresh per request, and that the
 * header Next reads while rendering is the same one the browser enforces.
 */

function run(path = "/clients") {
  const response = proxy(new NextRequest(`https://practice.terioscoach.com${path}`));
  return response.headers.get("Content-Security-Policy") ?? "";
}

function directive(csp: string, name: string): string {
  const found = csp
    .split(";")
    .map((part) => part.trim())
    .find((part) => part === name || part.startsWith(`${name} `));
  expect(found, `the policy has no ${name} directive`).toBeDefined();
  return found!;
}

describe("practice dashboard CSP", () => {
  it("mints a fresh nonce for every request", () => {
    const nonces = new Set(
      Array.from({ length: 8 }, () => /'nonce-([^']+)'/.exec(run())?.[1]),
    );

    // A nonce reused across requests is a nonce an attacker can read out of
    // one page and put in an injected tag on the next.
    expect(nonces.size).toBe(8);
    for (const nonce of nonces) {
      expect(nonce).toMatch(/^[0-9a-f]{32}$/);
    }
  });

  it("gives Next the same policy the browser will enforce", () => {
    const response = proxy(new NextRequest("https://practice.terioscoach.com/clients"));
    // The request-header overrides ride back on the response under this
    // prefix; that copy is what the renderer sees.
    const forRenderer = response.headers.get("x-middleware-request-content-security-policy");
    const nonce = response.headers.get("x-middleware-request-x-nonce");
    const forBrowser = response.headers.get("Content-Security-Policy");

    // Next stamps the nonce onto its script tags by reading it back out of
    // the request's copy of this header. If the two ever differ, the tags
    // carry a nonce the browser is not expecting and no script runs — a
    // blank dashboard, not a degraded one.
    expect(nonce).toBeTruthy();
    expect(forRenderer).toBe(forBrowser);
    expect(forBrowser).toContain(`'nonce-${nonce}'`);
  });

  it("keeps script-src strict", () => {
    const script = directive(run(), "script-src");

    // strict-dynamic is what makes this policy worth its cost: a script
    // without the nonce does not run, however it got onto the page, and
    // host allowlists — which an open redirect on an allowlisted origin
    // defeats — stop being consulted at all.
    expect(script).toContain("'strict-dynamic'");
    expect(script).toMatch(/'nonce-[0-9a-f]{32}'/);
    expect(script).not.toContain("'unsafe-inline'");
    // React uses eval in development to rebuild server error stacks. A
    // production bundle does not, and shipping the allowance anyway hands
    // an injected string an interpreter.
    expect(process.env.NODE_ENV).not.toBe("development");
    expect(script).not.toContain("'unsafe-eval'");
  });

  it("closes the directives whose absence is silent", () => {
    const csp = run();

    expect(directive(csp, "object-src")).toBe("object-src 'none'");
    // An injected <base> re-points every relative URL on the page.
    expect(directive(csp, "base-uri")).toBe("base-uri 'self'");
    // Without this, an injected form action posts a client's record
    // somewhere else.
    expect(directive(csp, "form-action")).toBe("form-action 'self'");
    expect(directive(csp, "frame-ancestors")).toBe("frame-ancestors 'none'");

    // The directive that turns an exfiltration attempt into a blocked
    // request, so a wildcard in it is the whole policy failing quietly.
    const connect = directive(csp, "connect-src");
    expect(connect).not.toMatch(/[\s']\*/);
    expect(connect).toContain("https://api.cloudinary.com");
    expect(connect).toMatch(/wss?:\/\/\S+/);
  });

  it("carries a nonce on documents but not on static assets", () => {
    // Matching _next/static would mint a nonce for every chunk and script
    // file, which carry no HTML and cannot use one.
    const source = config.matcher[0].source;
    expect(source).toContain("_next/static");
    expect(source).toContain("_next/image");

    // A prefetched document is never the one that executes — the real
    // navigation fetches it again, with its own nonce.
    const missing = config.matcher[0].missing.map((rule) => rule.key);
    expect(missing).toContain("next-router-prefetch");
    expect(missing).toContain("purpose");
  });
});
