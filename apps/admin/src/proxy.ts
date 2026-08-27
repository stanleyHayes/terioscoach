import { NextResponse, type NextRequest } from "next/server";

/**
 * Content Security Policy for the practice dashboard (PROD-06).
 *
 * This app gets the strict, nonce-based policy and the customer app does
 * not, because the thing that makes nonces expensive costs nothing here.
 * A nonce has to be minted per request, so every page it covers becomes
 * dynamically rendered — no static generation, no CDN caching. The
 * dashboard already sends `Cache-Control: private, no-store` on every
 * response (it holds one practitioner's client records), so there was
 * never anything cached to lose. The customer app's marketing pages are
 * statically generated and served from the edge, and giving that up for a
 * site with no HTML-rendering surface would be a bad trade; it gets a
 * static policy in `next.config.ts` instead.
 *
 * `'strict-dynamic'` is what makes this worth having: scripts loaded by a
 * nonce-carrying script inherit trust, so Next's chunk loader keeps
 * working, while an injected `<script>` — which cannot guess the nonce —
 * does not run. Host allowlists are ignored by browsers that understand
 * `'strict-dynamic'`, which is the point: they cannot be bypassed by an
 * open redirect or a JSONP endpoint on an allowlisted origin.
 *
 * Next applies the nonce to its own framework and bundle tags on its own,
 * by reading it back out of this header — nothing in `app/` has to thread
 * it through.
 */

/** Directives that do not depend on the request. */
function staticDirectives(): string[] {
  const api = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  // The signaling socket lives on the API origin; a WebSocket URL is not
  // covered by the http(s) entry, so both spellings are listed.
  const apiSocket = api.replace(/^http/, "ws");

  return [
    "default-src 'self'",
    // Styles carry no nonce on purpose. React writes layout through
    // `style` attributes (the week calendar positions every booking that
    // way, the report bars size themselves that way), and a nonce cannot
    // cover an attribute — only 'unsafe-inline' can. Adding a nonce here
    // as well would make browsers ignore 'unsafe-inline' and blank the
    // calendar. CSS injection without script execution is a far weaker
    // vector than the one script-src closes, so this is the accepted line.
    "style-src 'self' 'unsafe-inline'",
    // Cloudinary serves CMS media and client documents; data:/blob: cover
    // canvas exports and locally previewed uploads.
    "img-src 'self' data: blob: https://res.cloudinary.com",
    "font-src 'self'",
    // Uploads go browser→Cloudinary directly on a signed, folder-scoped
    // signature, so the upload endpoint is a first-class destination.
    `connect-src 'self' ${api} ${apiSocket} https://api.cloudinary.com`,
    // Video room: remote tracks arrive as blob-backed MediaStreams.
    "media-src 'self' blob:",
    "worker-src 'self' blob:",
    "object-src 'none'",
    // Without these two an injected tag could re-point every relative URL
    // on the page, or post the form the practitioner is filling in to
    // somewhere else entirely.
    "base-uri 'self'",
    "form-action 'self'",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "upgrade-insecure-requests",
  ];
}

export function proxy(request: NextRequest) {
  const nonce = crypto.randomUUID().replaceAll("-", "");
  // React reconstructs server error stacks in the browser with eval during
  // development. Production builds do not, so the escape hatch closes.
  const development = process.env.NODE_ENV === "development";

  const policy = [
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${development ? " 'unsafe-eval'" : ""}`,
    ...staticDirectives(),
  ].join("; ");

  // Next reads the nonce back out of the request's own copy of the header
  // when it renders, and stamps it onto the framework and bundle tags.
  const headers = new Headers(request.headers);
  headers.set("x-nonce", nonce);
  headers.set("Content-Security-Policy", policy);

  const response = NextResponse.next({ request: { headers } });
  response.headers.set("Content-Security-Policy", policy);
  return response;
}

export const config = {
  matcher: [
    {
      // Static assets and image optimization carry no HTML and need no
      // policy; paying for a nonce on them would only slow them down.
      source: "/((?!_next/static|_next/image|favicon.ico|icon.svg).*)",
      // A prefetched document is never the one that executes — the real
      // navigation fetches it again, with its own nonce.
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
