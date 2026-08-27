import type { NextConfig } from "next";

/**
 * Content Security Policy for the customer app (PROD-06).
 *
 * Static, not nonce-based, and that is a deliberate split from the practice
 * dashboard. A nonce must be minted per request, which makes every page it
 * covers dynamically rendered: no static generation, no edge caching. The
 * dashboard already sends `no-store` on everything, so it loses nothing and
 * takes the strict policy (see `apps/admin/src/proxy.ts`). This app is the
 * opposite case — nine statically generated marketing routes served from the
 * edge, with Lighthouse as an explicit deliverable — and it has no surface
 * that renders attacker HTML: there is no `dangerouslySetInnerHTML` in the
 * tree, and CMS bodies go through `Prose`, which renders text.
 *
 * So `script-src` keeps `'unsafe-inline'` for Next's boot script, and the
 * value comes from the directives that do not need a nonce and are not
 * decorative: `object-src`, `base-uri`, `form-action` and `frame-ancestors`
 * close plugin execution, base-tag hijacking, form redirection and framing,
 * and `connect-src` bounds where a script may send anything at all — which
 * is what turns an exfiltration into a blocked request.
 */
function contentSecurityPolicy(): string {
  const api = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  // The signaling socket lives on the API origin; a WebSocket URL is not
  // covered by the http(s) entry, so both spellings are listed.
  const apiSocket = api.replace(/^http/, "ws");
  // React reconstructs server error stacks in the browser with eval during
  // development. Production builds do not, so the escape hatch closes.
  const development = process.env.NODE_ENV === "development";

  return [
    "default-src 'self'",
    `script-src 'self' 'unsafe-inline'${development ? " 'unsafe-eval'" : ""}`,
    // React writes layout through `style` attributes, which no nonce or
    // hash can cover — only 'unsafe-inline'.
    "style-src 'self' 'unsafe-inline'",
    // Blog covers are arbitrary remote URLs entered in the CMS, restricted
    // to http(s) in the domain rather than to a host list. data:/blob:
    // cover the signature pad's canvas export and local upload previews.
    "img-src 'self' data: blob: https:",
    "font-src 'self'",
    `connect-src 'self' ${api} ${apiSocket}`,
    // Video room: remote tracks arrive as blob-backed MediaStreams.
    "media-src 'self' blob:",
    "worker-src 'self' blob:",
    "object-src 'none'",
    // Without these two an injected tag could re-point every relative URL
    // on the page, or post a client's intake form somewhere else entirely.
    "base-uri 'self'",
    "form-action 'self'",
    // Paystack and Stripe checkout are a top-level navigation, not a frame.
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "upgrade-insecure-requests",
  ].join("; ");
}

const nextConfig: NextConfig = {
  poweredByHeader: false,
  compress: true,
  async headers() {
    return [{ source: "/:path*", headers: [
      { key: "Content-Security-Policy", value: contentSecurityPolicy() },
      { key: "X-Content-Type-Options", value: "nosniff" },
      { key: "X-Frame-Options", value: "DENY" },
      { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
      { key: "Permissions-Policy", value: "geolocation=(), microphone=(self), camera=(self), payment=()" },
      { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
      { key: "X-DNS-Prefetch-Control", value: "off" },
    ] }];
  },
};

export default nextConfig;
