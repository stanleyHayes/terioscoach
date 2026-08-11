# Terios Wellness Spa — Digital Practice Platform
## Agent Execution Plan

Source scope: `Terios_Wellness_Implementation_Engagement_Proposal.pdf` (XCreativs Technologies, 9 Aug 2026).
One platform, three layers: **Public Website** + **Client Portal** (customer-facing app) and **Practice Dashboard** (admin app), sharing one Go API, one MongoDB database, one brand.

---

## 1. Confirmed Decisions

| Decision | Choice |
|---|---|
| Backend | One Go API, hexagonal architecture (domain / ports / adapters), RBAC (`client`, `practitioner`) |
| Database | MongoDB (Atlas), daily backups, encryption at rest |
| Email | Resend (confirmations, reminders, enquiries, feedback delivery) |
| Media & documents | Cloudinary (signed uploads, client documents, CMS images) |
| Payments | Paystack (cards + mobile money, international clients) |
| Video | Raw WebRTC build — own WebSocket signaling in the Go API + self-hosted TURN (coturn) |
| Backend deploy | Render Blueprint (`render.yaml`, IaC) |
| Frontend deploy | Vercel — **two separate projects**: customer app and admin app |
| Frontend | Next.js (App Router), TypeScript, latest stable |
| Quality | SonarQube quality gates enforced in CI for all three codebases |
| Versions | Latest stable of every dependency at scaffold time; pin exact versions in lockfiles |

## 2. Deployment Topology

```
terios-web (Vercel)        terios-admin (Vercel)         ← two separate Next.js apps
Public site + Client portal   Practice dashboard
        \                        /
         \                      /
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
| **Frontend Agent — Customer** | `terios-web`: public website + client portal |
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
| FND-06 | Resend: domain verification, transactional template set (confirm, remind, reschedule, enquiry, feedback) | DevOps | FND-02 | In Progress — 5 brand templates written (`design/email/`); domain verification needs Resend credentials |
| FND-07 | Cloudinary: account, signed-upload presets, folder policy (client docs vs CMS media) | DevOps | — | Not Started |
| FND-08 | `render.yaml` Blueprint: api service, coturn service, env groups, health checks, auto-deploy | DevOps | FND-01 | In Progress |
| FND-09 | Vercel projects `terios-web` + `terios-admin`: env wiring, preview deployments, API base URL config | DevOps | FND-01 | Not Started |
| FND-10 | CI (GitHub Actions): build, test, lint for all three codebases + SonarQube scan with blocking quality gate | DevOps | FND-01 | In Progress — workflows + sonar configs written; live gate needs SONAR_TOKEN |

## 6. Phase 2 — Public Website (`terios-web`, public area)

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| WEB-01 | App shell: brand theme tokens, layout, typography, custom nav/footer | FE-Customer | FND-03 | Done |
| WEB-02 | Home + brand story page | FE-Customer | WEB-01 | Done |
| WEB-03 | About & approach page (client-supplied material, refined) | FE-Customer | WEB-01 | Done |
| WEB-04 | Services pages with live pricing from API | FE-Customer | WEB-01, BE-03 | Done |
| WEB-05 | Blog (listing, article, categories) reading from CMS API | FE-Customer | WEB-01, BE-12 | Not Started |
| WEB-06 | FAQ: structured, searchable (custom search UI) | FE-Customer | WEB-01, BE-12 | Not Started |
| WEB-07 | Testimonials display (approved only) | FE-Customer | WEB-01, BE-12 | Not Started |
| WEB-08 | Contact / enquiry form → dashboard + email | FE-Customer | WEB-01, BE-13 | Not Started |
| WEB-09 | Work With Me conversion page | FE-Customer | WEB-04 | In Progress |
| WEB-10 | SEO (metadata, sitemap, OG), analytics, performance/Lighthouse pass | FE-Customer | WEB-02–09 | Not Started |

## 7. Phase 3 — Practice Core (backend + `terios-admin`)

### 7a. Backend

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| BE-01 | AuthN/AuthZ: register, login, refresh tokens, Argon2id hashing, JWT, RBAC middleware (`client` / `practitioner`) | Backend | FND-04, FND-05 | Done |
| BE-02 | Auth hardening: rate limiting, brute-force lockout, session hijack protections | Backend | BE-01 | Not Started |
| BE-03 | Services & pricing CRUD (dashboard-controlled, instantly public) | Backend | BE-01 | Done |
| BE-04 | Availability engine: working hours, session lengths, buffers, timezone-safe slot generation | Backend | BE-01 | Done |
| BE-05 | Booking engine: slot hold, conflict prevention, reschedule/cancel rules, booking lifecycle | Backend | BE-03, BE-04 | Done |
| BE-06 | Paystack adapter: charge at booking, webhooks, refunds, payment records per client | Backend | BE-05 | In Progress |
| BE-07 | Client records: profile, history, documents, forms, payments — strict ownership scoping | Backend | BE-01 | Not Started |
| BE-08 | Session notes: private notes vs shared feedback split | Backend | BE-07 | Not Started |
| BE-09 | Notification service (Resend): booking confirmations, automated session reminders (scheduler), reschedule notices | Backend | BE-05, FND-06 | Not Started |
| BE-10 | Form builder + digital signatures: intake/consent forms, attach to booking or send direct, signed storage | Backend | BE-07 | Not Started |
| BE-11 | Documents: Cloudinary signed upload/download, client-scoped access | Backend | BE-07, FND-07 | Not Started |
| BE-12 | CMS API: pages, blog posts, FAQs, testimonials (approve-before-publish) | Backend | BE-01 | Not Started |
| BE-13 | Enquiry inbox API (form → dashboard + Resend notification) | Backend | BE-01, FND-06 | Not Started |
| BE-14 | Reviews: client submission, practitioner moderation, publish to site | Backend | BE-07 | Not Started |
| BE-15 | Reporting API: sessions, bookings ahead, income by service/period, content engagement | Backend | BE-05, BE-06 | Not Started |

### 7b. Admin App (`terios-admin`)

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| ADM-01 | Admin shell: login (brand-styled), layout, route guards | FE-Admin | FND-03, BE-01 | Done (v1 scope cut: no sidebar-collapse/mobile-drawer variants yet) |
| ADM-02 | Calendar & scheduling: custom calendar component, availability/buffer editor, booking states | FE-Admin | ADM-01, BE-04, BE-05 | In Progress |
| ADM-03 | Client records UI: full client file (details, sessions, notes, forms, docs, payments, feedback) | FE-Admin | ADM-01, BE-07 | Not Started |
| ADM-04 | Session notes & feedback composer (private vs shared toggle) | FE-Admin | ADM-03, BE-08 | Not Started |
| ADM-05 | Services & pricing manager | FE-Admin | ADM-01, BE-03 | Done |
| ADM-06 | Payments & earnings views (per client, per service, over time) | FE-Admin | ADM-01, BE-06 | Not Started |
| ADM-07 | CMS UI: pages, blog editor, FAQ manager, testimonial moderation, Cloudinary image picker | FE-Admin | ADM-01, BE-12 | Not Started |
| ADM-08 | Form builder UI: field editor, assign to booking/client, view signed submissions | FE-Admin | ADM-01, BE-10 | Not Started |
| ADM-09 | Enquiry inbox UI | FE-Admin | ADM-01, BE-13 | Not Started |
| ADM-10 | Review moderation UI | FE-Admin | ADM-01, BE-14 | Not Started |
| ADM-11 | Reporting dashboard: charts (custom-styled), sessions/income/bookings/content | FE-Admin | ADM-01, BE-15 | Not Started |

## 8. Phase 4 — Client Experience (portal in `terios-web` + video)

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| CX-01 | WebRTC signaling server: authenticated WebSocket, session-bound rooms, join-window enforcement | Backend | BE-05, BE-02 | Not Started |
| CX-02 | Cloudflare Calls TURN: create TURN key, wire API-side credential endpoint, connectivity test harness | DevOps | FND-08 | Not Started |
| CX-03 | Portal auth screens + account area shell | FE-Customer | WEB-01, BE-01 | Done |
| CX-04 | Portal bookings: book new, view upcoming, reschedule within rules | FE-Customer | CX-03, BE-05 | In Progress |
| CX-05 | Portal video room: raw WebRTC client (custom UI — no native call chrome), one-click join, reconnect handling | FE-Customer | CX-01, CX-02, CX-04 | Not Started |
| CX-06 | Admin video room: start session from dashboard, session record attaches to client file | FE-Admin + Backend | CX-01, ADM-02 | Not Started |
| CX-07 | Portal forms & signatures: complete, sign (custom signature pad), submit | FE-Customer | CX-03, BE-10 | Not Started |
| CX-08 | Portal session history + shared feedback & resources | FE-Customer | CX-03, BE-08 | Not Started |
| CX-09 | Portal documents library | FE-Customer | CX-03, BE-11 | Not Started |
| CX-10 | Portal payment history + pay for new bookings | FE-Customer | CX-03, BE-06 | Not Started |
| CX-11 | Portal review submission | FE-Customer | CX-03, BE-14 | Not Started |

## 9. Phase 5 — Launch

| ID | Task | Agent | Depends On | Status |
|---|---|---|---|---|
| LCH-01 | Security hardening pass: OWASP review, headers, CORS, rate limits, secrets audit, WebRTC room isolation test | QA & Security | All BE + CX | Not Started |
| LCH-02 | E2E suite: book → pay → remind → video → notes → feedback → review, across both apps | QA & Security | CX-11, ADM-11 | Not Started |
| LCH-03 | SonarQube gates green across api/web/admin; coverage threshold met | QA & Security | LCH-02 | Not Started |
| LCH-04 | Atlas daily backups verified + restore drill | DevOps | FND-05 | Not Started |
| LCH-05 | Content placement: client copy, testimonials, service descriptions, images — refined and loaded via CMS | Design | ADM-07 | Not Started |
| LCH-06 | Load/performance pass: booking concurrency, video room stress, Lighthouse | QA & Security | LCH-02 | Not Started |
| LCH-07 | Practitioner training & handover runbook | Program | All above | Not Started |
| LCH-08 | Domain cutover (Google-registered domain → Vercel/Render), SSL, go-live | DevOps | LCH-01–06 | Not Started |
| LCH-09 | Post-launch monitoring, uptime alerts, settling-in support period | DevOps + QA | LCH-08 | Not Started |

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
4. **Session video length / recording** — raw WebRTC build assumes live-only, no recording (recording is a scope change).
5. **Reminders** — email-only via Resend per current scope; SMS/WhatsApp reminders would be a scope addition.
