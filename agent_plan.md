# Terios Wellness Spa — Digital Practice Platform
## Agent Execution Plan

Source scope: `Terios_Wellness_Implementation_Engagement_Proposal.pdf` (XCreativs Technologies, 9 Aug 2026).
One platform, three layers: **Public Website** + **Client Portal** (customer-facing app) and **Practice Dashboard** (admin app), sharing one Go API, one MongoDB database, one brand.

---

## 1. Confirmed Decisions

| Decision | Choice |
|---|---|
| Backend | One Go API, hexagonal architecture (domain / ports / adapters), RBAC (`client`, `practitioner`, permission-scoped `staff`) |
| Database | MongoDB (Atlas), daily backups, encryption at rest |
| Email | Resend (confirmations, reminders, enquiries, feedback delivery) |
| Media & documents | Cloudinary (signed uploads, client documents, CMS images) |
| Payments | Paystack (cards + mobile money, international clients) |
| Video | Raw WebRTC build — own WebSocket signaling in the Go API + self-hosted TURN (coturn) |
| Backend deploy | Render Blueprint (`render.yaml`, IaC) |
| Frontend deploy | Vercel — **three separate projects**: public website, client portal and admin app |
| Frontend | Next.js (App Router), TypeScript, latest stable |
| Quality | SonarQube quality gates enforced in CI for the API and all three frontends |
| Versions | Latest stable of every dependency at scaffold time; pin exact versions in lockfiles |

## 2. Deployment Topology

```
terios-web (Vercel)   terios-portal (Vercel)   terios-admin (Vercel)
Public website         Client portal             Practice dashboard
        \                    |                    /
         \                   |                   /
          terios-api (Render)  ← one Go hexagonal API, RBAC
          WebRTC signaling (WS)
           |      |       |
        MongoDB  Resend  Cloudinary
        (Atlas)

        Cloudflare Calls TURN (managed) ← NAT traversal for raw WebRTC
```

## 3. Agent Roster (role-based)

| Agent | Responsibility |
|---|---|
| **Program Agent** | Scope control, dependencies, client reviews, phase sign-offs |
| **Design Agent** | Brand direction, design system, all custom UI (no native elements) |
| **Backend Agent** | Go hexagonal API: domain, ports, adapters, integrations, WebRTC signaling |
| **Frontend Agent — Customer** | `terios-web`: public website; `terios-portal`: client account and care experience |
| **Frontend Agent — Admin** | `terios-admin`: practice dashboard |
| **DevOps Agent** | Render Blueprint, Vercel, Atlas, coturn, CI/CD, SonarQube, backups |
| **QA & Security Agent** | Test strategy, E2E, hardening, SonarQube gates, launch checklist |

## 4. Global Rules

- **No native UI elements, ever.** No default browser `<select>`, `<input type="date">`, `<input type="file">`, `<dialog>`, native buttons/scrollbars, etc. Every interactive element is a custom-built, brand-styled component from the design system. Design Agent owns the component spec; both Frontend Agents implement against it.
- **Hexagonal discipline:** domain logic has zero framework/driver imports. MongoDB, Paystack, Resend, Cloudinary, WebSocket all live behind ports in adapters.
- **Client data isolation is non-negotiable:** every client-scoped query enforces ownership at the repository adapter; cross-client access is impossible by construction, not by convention.
- **Secrets** only in Render/Vercel env stores — never in the repo.
- **Status legend:** `Not Started` · `In Progress` · `Blocked` · `Done`.
- Every task ships with tests; nothing merges red on SonarQube.

---

## 5. Phase 1 — Foundation

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| FND-01 | Monorepo scaffold: `api/` (Go), `apps/web` (customer), `apps/admin`, shared lint/format configs, latest stable Go + Next.js pinned | DevOps | — | Done |
| FND-02 | Brand identity & design direction (palette, typography, voice, spa-evolution headroom) | Design | — | Done |
| FND-03 | Design system spec: full custom component library (buttons, inputs, date/time picker, calendar, file upload, modal, toast, signature pad) — zero native elements | Design | FND-02 | Done |
| FND-04 | Hexagonal API skeleton: domain / ports / adapters layout, router, config, structured logging, health checks | Backend | FND-01 | Done |
| FND-05 | MongoDB Atlas cluster, collections & index design, seed tooling | DevOps + Backend | FND-04 | In Progress — adapter/indexes/seed code done; Atlas cluster needs credentials |
| FND-06 | Resend: domain verification, transactional template set (confirm, remind, reschedule, enquiry, feedback) | DevOps | FND-02 | **Blocked — domain registered but DNS verification not started.** The Resend key is valid and the live domains API reports `terioscoach.com` as `not_started`. Publish the DKIM/SPF records and wait for `verified`, or point `RESEND_FROM` at a verified domain; otherwise confirmations, reminders and shared-feedback emails are not production-safe. |
| FND-07 | Cloudinary: account, signed-upload presets, folder policy (client docs vs CMS media) | DevOps | — | **Verified live** — signed upload accepted by Cloudinary and a forged signature rejected, both against the real account (`internal/integration`). Account exists, credentials work, folder policy holds. |
| FND-08 | `render.yaml` Blueprint: api service, coturn service, env groups, health checks, auto-deploy | DevOps | FND-01 | Done — every one of the 35 config variables is declared. **PORTAL_URL corrected**: it was set to the bare origin, and the email renderer appends to it, so every client email linked to a 404 on the marketing site. Pinned by `TestEveryLinkResolvesToARealRoute`, which checks produced links against the app's real route tree. |
| FND-09 | Vercel projects `terios-web` + `terios-portal` + `terios-admin`: env wiring, preview deployments, API base URL config | DevOps | FND-01 | Config and independent CI are written; project creation needs Vercel access. Each `apps/*/vercel.json` carries deployment/security policy; portal and admin send `X-Robots-Tag: noindex` and `no-store`. The three-project env/domain table and exact `ALLOWED_ORIGINS` value are in `design/go-live-runbook.md` §5. |
| FND-10 | CI (GitHub Actions): build, test, lint for the API and all three frontends + SonarQube scan with blocking quality gate | DevOps | FND-01 | In Progress — workflows + sonar configs written; live gate needs SONAR_TOKEN |

## 6. Phase 2 — Public Website (`terios-web`, public area)

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| WEB-01 | App shell: brand theme tokens, layout, typography, custom nav/footer | FE-Customer | FND-03 | Done |
| WEB-02 | Home + brand story page | FE-Customer | WEB-01 | Done |
| WEB-03 | About & approach page (client-supplied material, refined) | FE-Customer | WEB-01 | Done |
| WEB-04 | Services pages with live pricing from API | FE-Customer | WEB-01, BE-03 | Done |
| WEB-05 | Blog (listing, article, categories) reading from CMS API | FE-Customer | WEB-01, BE-12 | Done |
| WEB-06 | FAQ: structured, searchable (custom search UI) | FE-Customer | WEB-01, BE-12 | Done |
| WEB-07 | Testimonials display (approved only) | FE-Customer | WEB-01, BE-12 | Done |
| WEB-08 | Contact / enquiry form → dashboard + email | FE-Customer | WEB-01, BE-13 | Done |
| WEB-09 | Work With Me conversion page | FE-Customer | WEB-04 | Done |
| WEB-10 | SEO (metadata, sitemap, OG), analytics, performance/Lighthouse pass | FE-Customer | WEB-02–09 | Done — metadata/canonicals/OG, sitemap and robots (preview builds refuse indexing). Analytics: Vercel Analytics + Speed Insights, chosen because they set no cookies, so the site needs no consent banner; the portal and preview deployments are excluded. The Lighthouse *measurement* needs a served build and is folded into LCH-06. |

## 7. Phase 3 — Practice Core (backend + `terios-admin`)

### 7a. Backend

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| BE-01 | AuthN/AuthZ: register, login, refresh tokens, Argon2id hashing, JWT, RBAC middleware (`client` / `practitioner`) | Backend | FND-04, FND-05 | Done |
| BE-02 | Auth hardening: rate limiting, brute-force lockout, session hijack protections | Backend | BE-01 | Done |
| BE-03 | Services & pricing CRUD (dashboard-controlled, instantly public) | Backend | BE-01 | Done |
| BE-04 | Availability engine: working hours, session lengths, buffers, timezone-safe slot generation | Backend | BE-01 | Done |
| BE-05 | Booking engine: slot hold, conflict prevention, reschedule/cancel rules, booking lifecycle | Backend | BE-03, BE-04 | Done |
| BE-06 | Paystack adapter: charge at booking, webhooks, refunds, payment records per client | Backend | BE-05 | Done |
| BE-07 | Client records: profile, history, documents, forms, payments — strict ownership scoping | Backend | BE-01 | Done |
| BE-08 | Session notes: private notes vs shared feedback split | Backend | BE-07 | Done |
| BE-09 | Notification service (Resend): booking confirmations, automated session reminders (scheduler), reschedule notices | Backend | BE-05, FND-06 | Done |
| BE-10 | Form builder + digital signatures: intake/consent forms, attach to booking or send direct, signed storage | Backend | BE-07 | Done |
| BE-11 | Documents: Cloudinary signed upload/download, client-scoped access | Backend | BE-07, FND-07 | Done (code + policy complete; live account needs Cloudinary credentials) |
| BE-12 | CMS API: pages, blog posts, FAQs, testimonials (approve-before-publish) | Backend | BE-01 | Done |
| BE-13 | Enquiry inbox API (form → dashboard + Resend notification) | Backend | BE-01, FND-06 | Done |
| BE-14 | Reviews: client submission, practitioner moderation, publish to site | Backend | BE-07 | Done |
| BE-15 | Reporting API: sessions, bookings ahead, income by service/period, content engagement | Backend | BE-05, BE-06 | Done |

### 7b. Admin App (`terios-admin`)

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| ADM-01 | Admin shell: login (brand-styled), fixed independently scrolling sidebar, collapsible desktop groups, mobile drawer, topbar menus, route guards | FE-Admin | FND-03, BE-01 | Done |
| ADM-02 | Calendar & scheduling: custom calendar component, availability/buffer editor, booking states | FE-Admin | ADM-01, BE-04, BE-05 | Done — `WeekCalendar` built from scratch (no calendar library), `/availability` covers weekly windows, per-day buffers and time off, and every booking state transition is a separate explicit action. |
| ADM-03 | Client records UI: full client file (details, sessions, notes, forms, docs, payments, feedback) | FE-Admin | ADM-01, BE-07 | Done |
| ADM-04 | Session notes & feedback composer (private vs shared toggle) | FE-Admin | ADM-03, BE-08 | Done |
| ADM-05 | Services & pricing manager | FE-Admin | ADM-01, BE-03 | Done |
| ADM-06 | Payments & earnings views (per client, per service, over time) | FE-Admin | ADM-01, BE-06 | Done |
| ADM-07 | CMS UI: pages, blog editor, FAQ manager, testimonial moderation, Cloudinary image picker | FE-Admin | ADM-01, BE-12 | Done — one `/content` screen with four tabs. Publish and approve are separate buttons hitting their own routes; a save can never put content live. Images upload browser→Cloudinary on a signed, folder-scoped signature, never through the API. |
| ADM-08 | Form builder UI: field editor, assign to booking/client, view signed submissions | FE-Admin | ADM-01, BE-10 | Done — `/forms`, definitions and responses on separate tabs. Field keys are assigned once and never recomputed, so rewording a label cannot orphan an answer already recorded. Submissions are read-only with the server's integrity verdict shown. |
| ADM-09 | Enquiry inbox UI | FE-Admin | ADM-01, BE-13 | Done |
| ADM-10 | Review moderation UI | FE-Admin | ADM-01, BE-14 | Done |
| ADM-11 | Reporting dashboard: charts (custom-styled), sessions/income/bookings/content | FE-Admin | ADM-01, BE-15 | Done |

## 8. Phase 4 — Client Experience (`terios-portal` + video)

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| CX-01 | WebRTC signaling server: authenticated WebSocket, session-bound rooms, join-window enforcement | Backend | BE-05, BE-02 | Done |
| CX-02 | Cloudflare Calls TURN: create TURN key, wire API-side credential endpoint, connectivity test harness | DevOps | FND-08 | **Verified live** — the Cloudflare Realtime TURN key mints real credentials: 2 ICE entries with stun:, turn: and turns: URLs and base64 username/credential. A bad token is refused with 401 and the token never reaches the error text. Not yet proven between two real networks — that is the e2e video spec. |
| CX-03 | Portal auth screens + account area shell | FE-Customer | WEB-01, BE-01 | Done |
| CX-04 | Portal bookings: book new, view upcoming, reschedule within rules | FE-Customer | CX-03, BE-05 | Done |
| CX-05 | Portal video room: raw WebRTC client (custom UI — no native call chrome), one-click join, reconnect handling | FE-Customer | CX-01, CX-02, CX-04 | Done — **and now actually reachable.** The room component and its hook were complete and tested but imported by nothing: `/portal/sessions/[id]/room` did not exist, so the Join link every client was offered 404'd. Route added with its own tests; the id comes from the route, leaving returns to the sessions list. |
| CX-06 | Admin video room: start session from dashboard, session record attaches to client file | FE-Admin + Backend | CX-01, ADM-02 | Done |
| CX-07 | Portal forms & signatures: complete, sign (custom signature pad), submit | FE-Customer | CX-03, BE-10 | Done |
| CX-08 | Portal session history + shared feedback & resources | FE-Customer | CX-03, BE-08 | Done |
| CX-09 | Portal documents library | FE-Customer | CX-03, BE-11 | Done |
| CX-10 | Portal payment history + pay for new bookings | FE-Customer | CX-03, BE-06 | Done — **and now actually wired.** `book/page.tsx` ended at a confirmation screen with a `TODO(payments)`; nothing created the first payment record, so the Pay-now button on the Payments page could never appear and no booking could ever be paid for. Confirming now hands off to Paystack checkout. Booking and payment stay separate: a failed hand-off keeps the slot and offers payment from the portal. |
| CX-11 | Portal review submission | FE-Customer | CX-03, BE-14 | Done |

## 9. Phase 5 — Launch

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| LCH-01 | Security hardening pass: OWASP review, headers, CORS, rate limits, secrets audit, WebRTC room isolation test | QA & Security | All BE + CX | Done — `design/security-review.md`; CORS + security headers added, secrets audit clean; dependency scanning and alerting tracked as open items |
| LCH-02 | E2E suite: book → pay → remind → video → notes → feedback → review, across all three frontends | QA & Security | CX-11, ADM-11 | Done (API half runs now; browser half needs preview deployments) — `journey_test.go` drives the whole story over real HTTP with in-memory adapters and runs on every commit. `e2e/` uses distinct web, portal and admin origins, including a two-browser WebRTC test that asserts `bytesReceived > 0`, not just a visible `<video>`. It is gated on FND-05/FND-09 and a seeded practitioner account. |
| LCH-03 | SonarQube gates green across api/web/portal/admin; coverage threshold met | QA & Security | LCH-02 | Done bar the live gate (needs SONAR_TOKEN) — enforced coverage ratchets in CI: API 65% floor, web 65/67/60/64, portal 65/67/60/64 and admin 73/72/68/71. Added `govulncheck` and `npm audit --audit-level=high`. Ratchets rise, never fall. |
| LCH-04 | Atlas daily backups verified + restore drill | DevOps | FND-05 | Procedure written; needs an Atlas cluster — `design/go-live-runbook.md` §1, including why M0 cannot satisfy this task at all and what the drill has to prove (a booking, its payment, and a signed form still reporting `integrityOk`). |
| LCH-05 | Content placement: supplied copy, testimonials, service descriptions, portraits, blog images and logo masters refined and loaded through CMS-ready paths | Design | ADM-07 | Done — the supplied archive was fully inventoried; 28 optimized portraits, 20 blog images and six final SVG logo variants are retained. Page copy and initial posts are idempotently seeded; testimonials remain approve-before-publish. See `design/brand-assets.md`. |
| LCH-06 | Load/performance pass: booking concurrency, video room stress, Lighthouse | QA & Security | LCH-02 | Booking concurrency done — `concurrency_test.go` runs 32 simultaneous bookings of one slot under `-race`: exactly one 201, the rest a clean 409 `slot_unavailable`, plus repeated-tap, unrelated-slot and read-under-write cases. Video stress and Lighthouse need a deployment (FND-05/FND-09). |
| LCH-07 | Practitioner training & handover runbook | Program | All above | Done — `design/handover-runbook.md`, written for the practitioner rather than an engineer: what each screen is for, which actions cannot be undone and why, the private-vs-shared notes rule, and the three symptoms that should be reported rather than worked around. |
| LCH-08 | Domain cutover (Google-registered domain → Vercel/Render), SSL, go-live | DevOps | LCH-01–06 | Runbook written; needs the domain and DNS access — `design/go-live-runbook.md` §7: lower the TTL the day before, verify in a fixed order, switch Paystack to live keys last and refund the first real payment. Rollback is a DNS revert with nothing to undo in the database. |
| LCH-09 | Post-launch monitoring, uptime alerts, settling-in support period | DevOps + QA | LCH-08 | Alerting built (needs a live deployment to point a monitor at) — `internal/domain/ops` holds the thresholds as pure, testable rules; `GET /v1/admin/ops/health` reports notification backlog, retry-exhausted failures, lockout spikes and payment-verification failures. Always HTTP 200 so a monitor doesn't page for a mail backlog, and `unknown` when the counters can't be read — four zeroes would look exactly like health. Closes LCH-01's alerting gap. |

---

## 10. Cross-Phase Dependency Highlights

- `FND-03` (design system) blocks **all** frontend work — it is the critical path start.
- `BE-01` (auth/RBAC) blocks every protected backend feature and both apps' shells.
- Booking chain: `BE-04 → BE-05 → BE-06 (Paystack) → BE-09 (reminders)`.
- Video chain: `BE-05 → CX-01 (signaling)`, plus `CX-02 (coturn)`; `CX-05`/`CX-06` need both.
- Public site pages needing live data (`WEB-04/05/06/07`) depend on their CMS/service APIs, so backend CMS work runs early in Phase 2, not late.
- Phase 2 (public site) can ship and go live while Phases 3–4 are still in progress, per the proposal.

## 11. Open Items to Confirm

1. **Version pins** — "latest stable" recorded at scaffold time (FND-01); Go / Next.js / mongo-driver / Paystack & Cloudinary SDKs verified that day.
2. **Currencies** — default settlement currency(s) in Paystack (GHS base? USD for internationals?).
3. **Analytics** — privacy-friendly choice for the "website visits / content engagement" reporting (e.g. Plausible vs first-party collection into MongoDB).
4. **Session video length / recording** — recording exists as **client-local** capture (MediaRecorder → `.webm` download on the recorder's own machine, with a ● Rec indicator relayed to the other party). Server-side recording/storage remains a scope change (needs a media server).
5. **Reminders** — email-only via Resend per current scope; SMS/WhatsApp reminders would be a scope addition.

## 12. Product-wide redesign and review (12 Aug 2026)

| ID | Task | Status | Verification |
|---|---|---|---|
| UX-01 | Refresh the shared type system: Outfit for body/UI copy and Figtree for headings, titles, and wordmarks | Done | Fontsource dependencies are self-hosted in both apps; production builds load the new contract |
| UX-02 | Recompose the public shell and homepage with stronger hierarchy, asymmetric service layout, atmospheric depth, refined motion, and a premium closing/footer treatment | Done | Web production build + focused component/page tests |
| UX-03 | Carry the new surface, card, button, focus, selection, loading, empty, and responsive behavior through public, auth, portal, and admin routes via shared primitives and layouts | Done | Both production builds + app test suites |
| UX-04 | Redesign the practice shell for small screens and upgrade the overview into a useful command surface | Done | Responsive horizontal navigation added below `lg`; desktop sidebar retained |
| UX-05 | Code-review repairs: skip navigation in both apps, branded 404s, meaningful blog-cover alt text, sentence-case navigation, and removal of unused Fraunces dependency | Done | Lint, TypeScript, static route generation, targeted tests |

The redesign deliberately preserves the existing eucalyptus/clay/sand identity and functional routes. It changes the visual hierarchy and interaction system without migrating frameworks or rewriting completed business flows.

## 13. Complete UI revamp — second pass (12 Aug 2026)

The first redesign pass was rejected because it leaned too heavily on shared primitives and did not prove route-by-route coverage. This pass takes the larger visual risk requested: a deep-eucalyptus “living practice journal” system, applied individually across the public site, both authentication experiences, the complete client portal, and every practice workspace.

| ID | Task | Status | Evidence |
|---|---|---|---|
| REV-01 | Replace the visual foundation and both application shells | Done | Outfit body, Figtree titles, expanded palette tokens, rebuilt public/portal/admin navigation, footer and page-header systems |
| REV-02 | Individually redesign every public and authentication route | Done | Route matrix in `design/ui-revamp.md`; includes new legal routes linked from the footer |
| REV-03 | Carry the redesign through every client portal route and state | Done | New care-record navigation, route mastheads, connective page treatment and redesigned booking/session/form/document/payment/review surfaces |
| REV-04 | Carry the redesign through every admin workspace and state | Done | New command rail, top bar, editorial page headers, data surfaces, forms, tables and modals across all dashboard routes |
| REV-05 | Render at desktop and mobile sizes; run tests, lint, builds and API regression suite | Done | Desktop and 390px render review; web 284/284, admin 233/233, API suite green; both production builds pass; lint has zero errors |

## 14. Component identity, motion, header and footer polish (12 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| POL-01 | Replace the generic shared card, button, badge, icon-button and inline-link treatments | Done | Care-thread card edge, responsive spotlight, tactile inset actions, clay care-point, animated link rule and active feedback in both apps |
| POL-02 | Add page transitions and in-page motion without destabilising persistent chrome | Done | Route-group templates, viewport-triggered marketing sections, short stagger, GPU-safe transform/opacity and complete reduced-motion fallback |
| POL-03 | Redesign the public header and mobile navigation | Done | Floating segmented frame, custom leaf mark, active capsules and numbered full-screen mobile chapter menu |
| POL-04 | Redesign the public footer | Done | Editorial booking close, new brand lockup, practice-quality markers, horizontal link rails and responsive legal row |
| POL-05 | Replace and verify favicon assets in both applications | Done | Matching SVG + ICO assets; all four local endpoints return 200 with correct MIME types |
| POL-06 | Verify component behavior and production output | Done | Web 285/285, admin 233/233; both builds pass; scoped lint is clean; desktop and 390px browser review complete |

## 15. Booking-shell continuity and settled-to-floating navigation (12 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| NAV-01 | Preserve the public website shell through guest booking | Done | `/portal/book` renders the same public header instead of a disconnected slim portal header |
| NAV-02 | Give authenticated portal clients a direct website exit | Done | `Back to website` action is visible in the portal navigation at every width |
| NAV-03 | Replicate Kedland's settled-to-floating header behavior | Done | Full-width at page top; contracts after 48px; expands on return; requestAnimationFrame throttling and reduced-motion handling retained |

## 16. Semantic card redesign (12 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| CARD-01 | Recompose booking service choices into a branded, information-led control | Done | Number rail, care label, duration capsule, separated price/action zone, asymmetric silhouette, tactile hover, and accessible whole-card radio semantics |
| CARD-02 | Carry the choice-card language into the public service chooser | Done | `/work-with-me` uses the same visual grammar with link actions and a dark preselected state |
| CARD-03 | Create companion card families for other content types | Done | Record treatment applied to sessions, forms, documents, payments and review prompts; booking summary and public review cards receive distinct semantic variants |
| CARD-04 | Verify code quality and production output | Done | Scoped ESLint clean; production build passes; focused affected-path tests 13/13. Full parallel suite reached 282/290 with eight resource-related 5s timeouts, none assertion failures |

## 17. Terms and Privacy editorial redesign (12 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| LEGAL-01 | Replace the duplicated divider-list layouts with a shared legal-page composition | Done | New `LegalPage` component owns trust summary, review metadata, contents navigation, reading chapters and closing actions |
| LEGAL-02 | Give Terms and Privacy distinct content cues while preserving shared visual language | Done | Route-specific summaries, icons, cross-policy links and unchanged plain-language policy copy |
| LEGAL-03 | Make long-form legal reading responsive and accessible | Done | Semantic article/section/nav structure, anchor targets with sticky offset, mobile chapter rails and comfortable measure/leading |
| LEGAL-04 | Verify the new legal routes | Done | Focused component test passes; scoped ESLint and diff checks clean; production build statically generates `/terms` and `/privacy` |

## 18. Password recovery (12 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| AUTH-REC-01 | Add secure password-reset issuance and persistence | Done | 256-bit random tokens, SHA-256 hashes only in MongoDB, sparse unique index, one-hour expiry and uniform public responses prevent address enumeration |
| AUTH-REC-02 | Add reset completion and session safety | Done | One-time atomic token consumption, existing password policy, Argon2 hashing and account-wide refresh-session revocation |
| AUTH-REC-03 | Add recovery email and API routes | Done | Rate-limited `/v1/auth/forgot-password` and `/v1/auth/reset-password`; Resend message links to the customer reset route |
| AUTH-REC-04 | Build the complete customer recovery experience | Done | Login entry point, branded request/success screen, token-aware new-password form, mismatch/expiry handling and return-to-login path |
| AUTH-REC-05 | Verify recovery implementation | Done | Auth service/Mongo/HTTP packages pass; web API tests 16/16; scoped ESLint clean; production build generates both new routes |

## 19. Production hardening and SEO — first slice (12 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| PROD-01 | Add practitioner password recovery to the admin workspace | Done | Admin-origin reset email for practitioner accounts; branded request/reset states; existing one-time token, expiry, password policy and session-revocation controls reused |
| PROD-02 | Make frontend security headers deployment-portable | Done | Both Next configs now disable framework disclosure and enforce nosniff, frame denial, referrer, permissions, COOP and DNS-prefetch policy; Vercel manifests match |
| PROD-03 | Prevent indexing and caching of private/auth surfaces | Done | Admin emits metadata and response-header noindex/noarchive plus private no-store; customer portal/auth group emits noindex metadata; recovery routes excluded from robots |
| SEO-01 | Strengthen link-preview presentation | Done | Generated 1200×630 branded Open Graph image is automatically attached through Next metadata conventions and statically verified in production build |
| SEO-02 | Re-verify canonical, robots and sitemap controls | Done | Focused robots/sitemap suite 6/6; preview indexing remains opt-out and portal/recovery routes stay outside sitemap |

## 22. Production completion additions (30 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| PC-01 | Separate the client portal into an independently deployable app | Done | `apps/portal`; private headers and CI; marketing auth routes redirect to `app.terioscoach.com`; three-app production builds pass |
| PC-02 | Add account profile, password, preferences and onboarding controls | Done | API-backed profile/password changes revoke sessions; admin guided tour; admin and portal settings; route and service tests pass |
| PC-03 | Make video consultation entry points explicit | Done | Client navigation/overview/session CTAs and admin calendar labels point to the existing authenticated WebRTC rooms |
| PC-04 | Move blog authoring to dedicated Markdown write/preview pages | Done | `/content/posts/new` and `/content/posts/[id]/edit`; safe GFM rendering in the public article surface; editor tests pass |
| PC-05 | Redesign and disambiguate startup loading and public empty states | Done | Shared high-contrast branded splash in all frontends; services empty state now names the unpublished-data condition and provides an enquiry action |
| PC-06 | Add restrained public-site motion | Done | Staggered page-intro and existing section reveals use transform/opacity only and disable under reduced motion |
| PC-07 | Verify the completion slice | Done | Admin 409 tests and coverage gate; portal 258 tests and coverage gate; web 176 tests and coverage gate; workspace lint/build; Go test/vet; live desktop and 390px browser checks with no horizontal overflow |

## 20. Production content, admin access and interaction hardening (30 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| PROD-04 | Make MFA opt-in and provide scannable authenticator enrollment with segmented OTP entry | Done | Enrollment does not activate MFA until a valid TOTP is confirmed; QR and manual secret are shown; six single-character OTP fields support paste and focus advance |
| PROD-05 | Add staff user, role and permission management | Done | Owner-managed `/team`; staff accounts, named role presets, 12 granular permissions, disable control, generated one-time password, JWT claims and server-side route enforcement |
| PROD-06 | Fix the admin shell and mobile navigation behavior | Done | Sidebar and main canvas scroll independently within `h-dvh`; grouped navigation collapses; mobile drawer stays viewport-bound without horizontal page overflow; MFA actions do not wrap |
| PROD-07 | Replace typed availability dates/times with custom controls | Done | Custom month-grid date picker and 15-minute time picker with min-date protection, outside-click/Escape close behavior and focused interaction tests |
| PROD-08 | Standardize empty states | Done | Shared animated icon/title/description/action states across admin, marketing and portal routes; animation is disabled under `prefers-reduced-motion` |
| PROD-09 | Integrate the supplied brand archive | Done | 28 optimized portraits, 20 blog images, six final vectors and two legal templates retained; all 48 editorial images appear in the CMS picker; disposition documented in `design/brand-assets.md` |
| PROD-10 | Upgrade splash/loading and 404 states | Done | Branded full-viewport wellness-orbit splash for both apps; responsive public/admin 404 compositions with clear recovery actions and reduced-motion handling |
| PROD-11 | Add operational KPI snapshots | Done | Live monthly overview snapshot with four KPIs and an accessible activity graph; summary strips precede calendar, clients, services, enquiries, reviews and team detail views |
| PROD-12 | Repair responsive overlays, picker clipping, report spacing, and payment failure | Done locally; deploy pending | Explicit sidebar/backdrop/topbar/menu stacking and width guards verified at 390px; Availability cards permit picker overflow and the calendar flips above the field on wider screens; all-zero report series render a compact state instead of an empty chart; production Render logs traced the payments panic to a typed-nil service and the composition root now preserves the intended 503 with a regression test |
| PROD-13 | Restore the admin CI coverage ratchet after the new dashboard/team/video surfaces | Done | 409 tests pass; branch coverage is 68.02% against the unchanged 68% ratchet. Sonar runs only when its required secrets exist and uses the patched v6 action. |
| PROD-04 | Run first dependency and production-build gate | Done | npm audit reports 0 vulnerabilities in web/admin; both production builds pass; relevant API packages pass; local `govulncheck` binary unavailable but CI already runs it |
| PROD-05 | Live edge/TLS/email/Lighthouse verification | Blocked on deployment | Must be run against real origins; tracked in `design/security-review.md` open items |
| PROD-06 | Strict nonce-based browser CSP | Done | See §20 |

## 20. Production readiness pass (27 Aug 2026)

A review of the whole platform against what CI and a real browser actually
report, rather than what the plan claimed. Every item below was verified,
not assumed.

### 20a. Defects found and fixed

| ID | Defect | Fix | Evidence |
|---|---|---|---|
| FIX-01 | **Both frontend suites failed nondeterministically**, so `test:coverage` — the exact command CI runs — was red. Ten web and three admin tests timed out, none on an assertion: vitest's 5s default is shorter than the first render in a worker, and whichever test happens to be first in its file pays the jsdom + import + compile cost for all of them. | `testTimeout`/`hookTimeout` raised to 30s in both vitest configs. The ceiling is for a hung test, not a slow one — the same tests finish in ~600ms once warm. | Web 45 files / 350 tests green; admin 37 / 388 green |
| FIX-02 | **Concurrent 401s signed the user out.** A refresh token is one-time and the API revokes the whole session family when a spent one comes back (`auth.Service.Refresh`) — correct against theft, and fatal here, because expiry is never observed by one request alone. The portal mounts two or three authenticated reads at once and `/content` mounts four; fifteen minutes after sign-in they 401 together, each refreshed with the same token, and every request after the first looked exactly like a replay. The client was signed out of every device for opening a page. | Rotation is now single-flight in both apps' `api.ts`: concurrent callers await one refresh, and a caller whose snapshot has already been rotated past takes the newer tokens instead of presenting its dead one. `resetTokenRotation` is wired into both auth providers so tokens never cross a sign-out. | Three new tests per app, including four simultaneous 401s producing exactly one rotation |
| FIX-03 | **Abandoned-checkout payments were lost.** `Reinitialize` overwrote the gateway reference, but re-initializing does not cancel the checkout it replaces — the gateway page is still open in whichever tab the client left it in. Paying there produced a real charge whose webhook quoted a reference no record advertised, so `FindByReference` missed and the delivery was acknowledged with no changes. Money taken, booking still reading unpaid. | Superseded references are retained (`Payment.PreviousReferences`, capped at 20) and matched by both the Mongo adapter and the fake; a charge confirmed under one promotes it to the live reference, so a later refund quotes the transaction that was actually charged. | `TestWebhookOnAbandonedCheckoutStillRecordsThePayment`, `TestRefundQuotesTheChargedReference` — both verified to fail without the fix |
| FIX-04 | **`ALLOWED_ORIGINS` unset in production was a silent outage.** The API came up, passed every probe, and refused every browser call from both apps: CORS permitted no origin and the signaling socket refused every handshake. | Production now refuses to start without it, alongside the existing `MONGODB_URI` and JWT-secret guards. | New `internal/config` suite (the package had none): `TestProductionRequiresItsSecretsAndOrigins` |
| FIX-05 | **The password-reset token reached analytics.** `Analytics` excluded `/portal`, `/login` and `/register`, but the recovery routes shipped later (AUTH-REC-04, PROD-01) were added to `robots.ts` and not to this list — so `/reset-password?token=…` was reported as a page view with a live one-time credential in the URL. | One `PRIVATE_PATHS` list in `lib/seo`, consumed by both, so they cannot drift again; plus a `beforeSend` that strips the query string from every reported URL, which does not depend on anyone remembering to extend a list. | `Analytics.test.tsx` covers both recovery routes and the stripping |
| FIX-06 | **Every field error was announced twice.** Both forgot-password screens passed `error` to `TextInput` — which renders it as `role="alert"`, tied to the input — *and* rendered a second identical alert beside it. | The redundant paragraph removed from both; `TextInput`'s is the better one, being associated with the field via `aria-describedby`. | "announces the error exactly once" |
| FIX-07 | Admin `vercel.json` had drifted from `next.config.ts`: `Referrer-Policy` was the customer app's `strict-origin-when-cross-origin` rather than the dashboard's `no-referrer`, and `X-Robots-Tag` had lost `noarchive`. | Manifest realigned. | Diff |
| FIX-08 | ESLint linted the generated `coverage/` reports, reporting problems in code nobody wrote. | `coverage/**` added to both configs' ignores. | Both lints report zero problems |

### 20b. PROD-06 — browser CSP

Shipped as two policies, because the cost of a nonce falls very differently
on the two apps. The dashboard gets the strict nonce-based one; the customer
app gets a static one that keeps its statically generated marketing routes.
Full rationale in `design/security-review.md` §Open items 6.

Verified in a real browser against production builds of both apps: **16
routes, zero CSP violations, all hydrated.** That check is what caught the
breakage this task had been deferred over — with the nonce policy but no
`force-dynamic`, admin pages were prerendered, their script tags carried no
nonce, and `'strict-dynamic'` refused every one, giving a blank dashboard.

### 20c. Coverage gates

`test:coverage` had never passed in either app: the LCH-03 ratchets were
above what the suites achieved, by 7 points of branch coverage in admin.
Measured against a clean `HEAD` worktree to confirm this predated the pass.
Closed by testing what was genuinely untested rather than by lowering a
threshold — the client record page, both recovery screens in both apps, the
modal's focus trap, the submission viewer, the assign-form dialog, and the
`clients`/`inbox`/`insights`/`portal` API modules.

| Suite | Was | Now | Floor |
|---|---|---|---|
| API | 68.3% statements | 68.8% | 65% |
| Web | 63.6 / 59.0 / 64.2 / 66.8 | **67.6 / 61.4 / 68.5 / 69.1** | 64 / 60 / 67 / 65 |
| Admin | 69.2 / 61.0 / 69.6 / 72.5 | **76.3 / 68.4 / 75.9 / 78.4** | 71 / 68 / 72 / 73 |

(statements / branches / functions / lines)

Thresholds were left where they are rather than ratcheted up to the new
figures: admin's branch coverage clears its floor by 0.4 points, and raising
it now would trade a passing gate for a fragile one.

### 20d. Still open, and why

| Item | Why it cannot close here |
|---|---|
| FND-05, FND-09, LCH-04, LCH-08 | Need an Atlas cluster, a Vercel account, and DNS access |
| FND-06 | Resend reports `terioscoach.com` as registered but `not_started`; DKIM/SPF verification is required before relying on production email |
| FND-10, LCH-03 live gate | Needs `SONAR_TOKEN` |
| LCH-05 | The client's own copy, photographs and testimonials |
| PROD-05, LCH-06 (Lighthouse, video stress), LCH-09 monitor | Need real origins to measure |
| Penetration test | An adversarial exercise against a running system, not a code review |

One deliberate non-change: `use-video-room.ts` (1,026 lines) and `video.ts`
are duplicated near-verbatim across both apps, differing only in the auth
context's naming. Sharing them means a `packages/*` workspace and new Vercel
root directories — a deployment change, not a refactor, and not one to make
in the same pass as the fixes above. It is the largest piece of technical
debt left in the tree.

## 21. Production accounts, opt-in MFA, and release re-verification (30 Aug 2026)

| ID | Task | Status | Evidence |
|---|---|---|---|
| AUTH-MFA-01 | Provision the two production dashboard accounts without demo data | Done | `cmd/seed-production` is confirmation-gated, environment-only, idempotent, and preserves existing passwords/MFA; both requested practitioner accounts were created in the configured remote production database |
| AUTH-MFA-02 | Keep MFA opt-in | Done | Password login works before enrollment and while enrollment is pending; service test proves a code is required only after successful confirmation |
| AUTH-MFA-03 | Add scannable authenticator enrollment and segmented OTP entry | Done | Standard `otpauth://` QR, manual secret fallback, six individual numeric inputs with paste/keyboard support, and focused admin tests |
| AUTH-MFA-04 | Protect MFA lifecycle | Done | AES-256-GCM seed encryption under required `MFA_ENCRYPTION_KEY`; current-code verification for enable/disable; disabling revokes refresh sessions |
| PROD-07 | Refresh security and release gates | Done locally | Go 1.26.6; `go test -race ./...`; `go vet ./...`; `govulncheck ./...` zero reachable vulnerabilities; npm audits zero; web 350 tests + build; admin 391 tests + coverage + build; lint/typecheck/diff checks clean |

Repository readiness is green. Deployment-only gates in §20d remain open and
must not be represented as locally verified: live DNS/TLS/edge behavior, Resend
domain delivery, Atlas restore drill, Sonar live gate, Lighthouse/video stress,
monitor ownership, final client content, and a penetration test against the
running production origins.
