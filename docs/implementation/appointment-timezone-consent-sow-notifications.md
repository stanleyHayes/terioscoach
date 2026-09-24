# Appointment, consent, Statement of Work and notification updates

Date: 20 September 2026
Status: Code implemented 23 September 2026. Compilation/static checks recorded below; test execution and browser verification reserved for owner.
Audience: Any agent implementing or verifying these changes

## 1. Purpose and authority

Implement these four requirements in order:

1. Unify appointment timezone handling so a newly booked future session does not appear as past when another user or device loads it.
2. Ask whether the appointment is for someone under 18 and require a parent/guardian consent signature when it is.
3. Make the Statement of Work (SOW) a form the user completes and electronically signs.
4. Reliably notify clients and practitioners about changes requiring their attention.

Sections 3–9 retain the implementation specification; section 11 records the delivered code and verification boundary. The owner's 20 September request supersedes the earlier **unsigned SOW** requirement recorded under Agreements 2 and 7 in `agent_plan.md`. Preserve the historical completion evidence there; do not treat it as acceptance of the new requirement. Existing signatures, submissions, signed wording and PDFs must not be rewritten to appear newly signed.

Scope: Go API, MongoDB adapters, client portal, practitioner app, generated documents, transactional email and in-app notifications. Public website changes are limited to affected booking entry points. SMS and browser push are outside this request. Do not implement unrelated backlog items.

## 2. Decisions and assumptions

| Decision | Proposed contract | Confirmation / dependency |
|---|---|---|
| User-selected timezone | Each client and practitioner selects an IANA timezone inside the app. Persist it on their authenticated account and use it consistently across their screens and notifications. Store appointment instants in UTC. | Confirmed by owner on 20 September: configurable in the app; user selects timezone. No single fixed US timezone. |
| Scheduling versus display | Availability retains its authored timezone. Each user views the same appointment instant in their saved timezone. | Different users may see different clock times, but must see the same instant and lifecycle status. |
| Under 18 | Explicit booking-level checkbox, initially unchecked, plus required acknowledgement that participant details are accurate. Checked means guardian consent is required. | Eligibility is at the appointment date. Do not silently infer adult status for legacy records. |
| Account versus participant | Existing authenticated client remains the booking/account owner; participant name and minor status are separate. | No new minor account or self-registration flow is implied. |
| Guardian signing | Capture guardian identity, relationship, explicit consent and an electronic signature bound to this participant and document version. | Owner authorized an under-18 checkbox with parent/guardian name and signature, plus recommended wording editable in admin. Use a declared guardian signing within the authenticated account; no separate guardian account or email verification claim. |
| SOW signers | Client fills and signs; when for a minor, the guardian signs in the guardian role. | Practitioner countersignature remains limited to document types already requiring it unless the owner changes that rule. |
| Booking readiness | Reserve/checkout may proceed according to current payment rules; session readiness must remain blocked until required documents and guardian consent are complete. | Owner approved checkout while required documents block session access. |
| Notification channels | Persistent in-app events for affected participants; transactional email for requests and important changes needing attention. | Keep optional acknowledgements in-app to avoid duplicate email noise. |

Owner authorized implementation and recommended guardian wording in this session. This is an engineering plan, not a claim of legal sufficiency of any consent wording.

## 3. Phase 0 — Read the evidence and establish contracts

Read `agent_plan.md`, applicable `AGENTS.md`, `design/api-contract.md`, `design/design-system.md`, `design/in-app-notifications.md`, and the implementation files below. Frontend agents must read relevant installed Next.js documentation required by each app's `AGENTS.md`. Inspect repository state before editing; the planning checkout already contains unrelated modified and untracked files.

### Confirmed starting points

| Area | Current evidence | Implementation consequence |
|---|---|---|
| Booking classification | `apps/portal/src/lib/bookings.ts`, `splitBookings`, places non-confirmed bookings in past, including `pending_payment`. Paid creation uses that status in `api/internal/app/booking/service.go`. | Fix status grouping as well as timezone presentation. This is a confirmed independent explanation for a fresh booking appearing past; it does not establish every timezone failure's root cause. |
| Display timezone | Portal booking uses selectable/browser-derived timezone; portal dashboard and sessions derive browser timezone independently. Domain `api/internal/domain/booking/booking.go` does not persist a booking timezone. | Establish an explicit persisted user-timezone contract across all readers and writers. |
| Existing timezone work | Calendar 1–4 in `agent_plan.md` describe authored availability zones, selectable display zones and DST tests. | Preserve availability semantics and regression tests; do not reinterpret existing UTC timestamps. |
| SOW form | `apps/portal/src/components/portal/StatementOfWorkStep.tsx`; `api/internal/domain/agreement/statement_of_work.go`. Four existing fields: client name, effective date, initial term months, monthly fee. | Reuse the form; add execution/signature semantics instead of replacing it with static agreement text. |
| SOW policy | `api/internal/domain/agreement/agreement.go`, `api/internal/app/agreements/service.go`: SOW is excluded from client-signature policy and service status discovery. Existing submission endpoint is `POST /v1/agreements/{id}/submit`. | Update discovery, readiness and submission together; simply adding a signature input will leave an unreachable or rejected flow. |
| PDF and practitioner review | `api/internal/adapters/pdf/agreement.go`, `api/internal/app/agreements/archivist.go`, `apps/admin/src/components/clients/ClientAgreements.tsx`. | Add signed-form rendering and preserve legacy unsigned rendering. |
| Notifications | `api/internal/app/notifications/service.go`, `activity.go`, ports and Mongo repositories; both apps already have persistent notification feeds. | Extend and verify existing infrastructure; do not build a second bell/outbox. |
| Delivery risk | Application notifier calls follow successful business writes; queue/delivery failures are reported. In-app deduplication does not prove atomic business-event persistence or exactly-once email. | Test the write-to-outbox failure window and implement recovery. |

### Existing APIs and patterns to reuse

- Booking and agreement application services and domain validators; keep business rules out of HTTP handlers and React components.
- `Agreement.SubmitStatementOfWork(...)` for existing answer validation/snapshot construction, extended with explicit signed versus legacy submission semantics.
- `ports.Notifier`, `ports.ActivityNotice`, `ports.NotifyActivity(...)`, `NotificationJobRepository.Create/Update/ClaimDue`, `InAppNotificationRepository` for existing notification boundaries.
- `notifications.Service.DispatchDue(...)`, `recordInApp(...)` and existing retry policy; inspect them before adding new event kinds.
- Existing `GET /v1/notifications` and recipient-scoped read operations; retain ownership enforcement.
- Existing signature capture and document archival components; inspect signatures/types before reuse. New fields/endpoints described below are proposals, not APIs that already exist.

Phase 0 deliverables: a reproducible booking trace (selected slot → request → persisted instant → response → classification), owner decisions, API/data contract changes, event matrix and migration inventory. Use synthetic accounts and record app/browser/server timezone and payment status. Verify root causes before coding a time conversion fix.

## 4. Phase 1 — Unified appointment time and correct status grouping

### Tasks

**TZ-01 — Persisted user preference.** Add or extend authenticated account settings with a validated IANA timezone. Provide a searchable, branded selector in portal and practitioner settings, plus the booking flow. Selection saves immediately with visible confirmation, persists after login/reload and across devices, and synchronizes other screens. A booking selector change updates the same preference, not an unrelated temporary variable. During first use, browser detection may suggest a zone, but the user explicitly selects/confirms it; thereafter the saved preference wins over browser timezone and URL parameters. Define loading/error states so screens do not briefly render in a different zone. Use existing settings/API patterns after inspecting them. Distinguish calendar dates (`YYYY-MM-DD`, such as SOW effective date) from appointment instants.

**TZ-02 — Time persistence and conversion.** Persist/return `startAt` and `endAt` as unambiguous UTC/RFC3339 instants. Record the booking-time selected zone as audit context; it does not replace the account preference for current display. Render appointment times using the authenticated user's saved zone across dashboard, booking, sessions, rescheduling, admin calendar/details and relevant exports. Format each recipient's email independently using their saved zone; resolve it at delivery for future reminders and retain the rendered-zone evidence. Previously sent messages are immutable. Show a zone label with event-date offset/abbreviation. Store IANA names, never fixed EST/PST offsets. Changing a preference never reschedules appointments or rewrites availability rules.

**TZ-03 — Scheduling boundaries.** Generate display day keys and day/week query bounds in the user's saved zone, converting boundaries to UTC for API queries. Retain availability rules' authored IANA zones; changing an availability rule's zone is a separate, explicit schedule edit. Reject nonexistent DST wall times and disambiguate repeated times with explicit instants/offsets. Server validates slot availability and conflicts against instants. Prefer selecting server-issued slot instants over reconstructing timestamps from formatted strings. Preserve intentional legacy schedule fallbacks until a separately verified migration resolves them; removing an Accra fallback indiscriminately could move historical schedules.

**TZ-04 — Session grouping.** Replace the assumption that anything not confirmed is past. Separate awaiting-payment/action-required, upcoming, in-progress and history as appropriate. Confirmed sessions are upcoming before start, in progress from start through before end, and past at/after end; completed/cancelled/no-show are terminal history. Payment-required sessions are visibly pending until expired or cancelled, never labelled completed/past merely because unconfirmed. Define pending expiry on the server. Compare instants, not formatted dates or local midnight. Use authoritative server time or an explicit clock-skew strategy for boundary-sensitive UI.

**TZ-05 — Existing data.** Dry-run audit timestamps and missing zone metadata. Preserve all already-valid instants. Backfill only provenance-supported zone metadata; unknown provenance stays explicit. Do not shift all records by a guessed offset. Produce counts, sample synthetic cases, backup/rollback and an idempotent migration before any data writes.

### Verification and guards

- Freeze time and load the same future booking in New York, Chicago, Los Angeles, Honolulu, Accra and UTC browser contexts. With the same saved account zone, show the same primary time and group everywhere; with different selected zones, show correctly converted times but identical instants and groups.
- Reproduce a new paid booking still awaiting webhook/payment: it appears as pending, never historical solely due to status. Verified payment promotes it correctly.
- Test just before start, exactly start, just before end, exactly end, cancellation, no-show and expired payment hold.
- Test US spring-forward nonexistent time, fall-back duplicated time, civil midnight, cross-zone date changes, week bounds and rescheduling.
- Assert identical instants across request/storage/response and recipient-specific notice text; browser timezone changes cannot alter the booked instant. Test saved preference across devices, booking/settings synchronization, an explicit preference change, invalid preference rejection and different client/practitioner zones.
- Reject offset-free instant strings and invalid IANA names. Never parse a display-formatted date to recover an instant.

References: booking files in phase 0; portal/admin timezone helpers and tests discovered there; notification `bookingData`, `formatTime`, `formatDate` and email templates.

## 5. Phase 2 — Under-18 participant and guardian consent

### Tasks

**MIN-01 — Booking participant UI and contract.** Add an accessible, design-system checkbox: “This appointment is for someone under 18.” Persist participant name, explicit minor declaration and guardian-required state with the booking. For a checked booking, collect guardian full name, relationship and contact details needed by the approved signing flow. Show a practitioner badge and consent status. Keep participant identity separate from the account holder. Do not collect full date of birth unless the approved workflow needs it.

**MIN-02 — Server-side enforcement.** Extend booking domain, HTTP request/response types, repositories and both frontend types. Validate guardian requirements server-side; hidden fields or unchecked UI controls cannot bypass them. A consent must match booking/participant, document key/version and signer role. Enforce the agreed readiness gate in all booking/session entry paths, including practitioner-created bookings and rescheduling where applicable.

**MIN-03 — Signing and evidence.** Redesign signature lookup and uniqueness: current Mongo signatures are unique by `(clientId, agreementId)` and can return an old account-level record before new input is validated. Introduce explicit participant/intake context, document version and signer-role coverage so one parent's signature cannot cover a different child. Existing pre-booking agreement gates need a durable intake/context ID that is later attached to the booking; avoid requiring an already-created booking to pass its own creation gate. Migrate indexes compatibly without deleting evidence.

 Reuse current signature capture and immutable document snapshot patterns. Record guardian name, relationship, signer role, consent wording/version, signature representation, server signing timestamp and authenticated actor. Distinguish guardian identity declaration from verified identity. If external guardian signing is selected, add narrowly scoped expiring single-use links, safe resend/revocation and authorization tests; do not email account credentials or expose the full client record.

**MIN-04 — Document and lifecycle.** Add the guardian block to the relevant consent/executed documents, including a signed SOW when applicable. Preserve the exact signed answers/wording. Changing participant or minor status invalidates readiness for the affected consent and creates a new execution/version rather than editing evidence. Rescheduling must re-evaluate age-at-appointment declarations when applicable. Do not remove the original guardian evidence when a participant later becomes an adult.

**MIN-05 — Legacy bookings.** Existing unknown-age records stay “not recorded,” not falsely adult or guardian-signed. Obtain a declaration for future appointments at the next relevant workflow step; expose unresolved readiness to practitioners. Historical sessions remain readable and retain original documents.

### Verification and guards

Test adult and minor paths, account-holder versus participant distinction, keyboard checkbox use, missing guardian fields, tampered requests, wrong participant/booking consent, duplicate submission, denied cross-client access, changed declaration, expired external link if implemented, and refresh/reload persistence. Generated PDF and practitioner view must identify the actual signer role and consent version. No session may pass the agreed gate with a required guardian signature missing.

References: existing agreement domain/signature flow, HTTP agreement adapter, Mongo agreement repository, PDF renderer, `AgreementStep.tsx`, booking gate tests. Inspect existing guardian-related PDF text first; a printed blank signature block alone does not implement consent capture.

## 6. Phase 3 — Fillable and electronically signed Statement of Work

### Tasks

**SOW-01 — Reachable requirement.** Update document discovery/status logic so required `holistic_sow` and `nurse_sow` appear in the booking/document flow. Replace the unsigned-only policy with a versioned completed-and-signed requirement. Preserve countersignature policy for other agreements.

**SOW-02 — Real form.** Reuse `StatementOfWorkStep.tsx` and its four existing answers. Compare both supplied SOW source documents and seeded text before finalizing the field schema; list every blank that must be completed and do not invent additional commercial terms. Clarify whether monthly fee is client-entered or prefilled/read-only; do not let an editable answer override actual payment pricing. Use date-only validation for effective date and bounded numeric/currency validation. Provide validation, review of completed answers and explicit electronic-signature acknowledgement followed by a signature action.

**SOW-03 — Atomic signed submission.** Consult `api/internal/domain/form/submission.go` for existing typed/drawn signature, UTC timestamp and answer-integrity patterns, plus portal `SignaturePad.tsx`.

 Extend the existing submit contract, or add a documented versioned signed-submit operation if required for compatibility. Validate all required answers and signature together on the server. Persist an immutable answer/wording/version snapshot and real signature evidence with signer role/time. Readiness becomes complete only after persistence succeeds. Retry must return the same execution, not create duplicate signatures/events. If a draft is supported, draft saving never counts as signed and never triggers a signed notification.

**SOW-04 — Read, archive and notify.** Show completed form answers plus signature in portal documents, practitioner client record and generated PDF. Include guardian role where required. Use archival retry patterns so temporary PDF failure cannot lose the signed snapshot or create a false second signature. Keep completed versus PDF-pending states honest. Notify the practitioner once when signed; acknowledge to the client.

**SOW-05 — Historical compatibility.** Render existing unsigned submissions as “Submitted — unsigned (legacy)” with no fabricated signature date/name. New requirements apply prospectively; if an upcoming legacy booking needs signature, request a new execution explicitly. Do not overwrite archived PDFs or reuse a previous unrelated client's/participant's signature. Preserve old endpoint compatibility deliberately and ensure an old unsigned request cannot satisfy a new signed requirement.

### Verification and guards

Cover both SOW keys, discovery visibility, each required field, invalid date/term/fee, blank signature, guardian signer role, duplicate/retried submission, refreshed document visibility, immutable answer snapshot, wrong-client access, PDF failure/retry and legacy unsigned rendering. Browser test must complete the real form and sign it; a static agreement plus checkbox does not satisfy this requirement.

References: phase 0 SOW files; `api/internal/app/booking/agreement_gate_test.go`; agreement domain/service/HTTP/Mongo tests; `ClientAgreements.test.tsx`; portal agreement component tests. Update comments and labels saying SOW has no signature only where they describe the new version, retaining legacy explanations.

## 7. Phase 4 — Notification coverage and dependable delivery

Extend `design/in-app-notifications.md` with an implementation-verified matrix. Existing coverage is a starting point, not proof of current production delivery.

| Persisted event | Client | Responsible practitioner | Destination / email rule |
|---|---|---|---|
| Booking created, payment pending | In-app + payment-required email | In-app new booking | Exact booking/payment action; never claim confirmed |
| Booking confirmed/payment verified | In-app + email | In-app + email | Booking/session details |
| Reschedule/cancel requested | In-app acknowledgement | In-app + email | Review the exact request |
| Request resolved; booking moved/cancelled | In-app + email | In-app acknowledgement; email if another actor changed it | Final booking state; old and new times when moved |
| Form/SOW/consent assigned or signature requested | In-app + email | In-app acknowledgement | Exact outstanding document |
| Form submitted; agreement/SOW/guardian consent signed | In-app receipt | In-app + email | Exact client/document execution |
| Practitioner countersigned | In-app + email | In-app receipt | Executed document |
| Required document rejected or revision requested, if supported | In-app + email | In-app receipt | Corrective action and safe reason |
| Shared document/resource/recording ready | In-app; email when action is required | In-app receipt | Authorized document/resource |
| Payment failed/expired/refunded | In-app + email for actionable/material change | In-app + email if intervention required | Payment/booking details |
| Session reminder | In-app + email at due time | In-app + email at due time | Session and recipient's saved-zone time |
| Completed/no-show; review/enquiry events | Preserve existing role-appropriate coverage | Preserve existing role-appropriate coverage | Relevant record; no private content disclosure |

Recipient means the authenticated client/account holder or assigned practitioner, not any email supplied in a request. A separate guardian receives only the permitted signing request/result for their scope if that workflow is selected. Do not expose private clinical notes or unrelated client details in emails or notifications.

### Tasks

**NOT-01 — Trace every event.** Map each matrix row to application mutation, event/outbox kind, recipients, template, deep link, tests and delivery status. Mark unsupported transitions explicitly instead of inventing events. Audit current practice-address fallback against assigned-practitioner routing. A generic first practitioner is not sufficient when a booking has a responsible practitioner.

**NOT-02 — Durable event creation.** Close the successful-business-write/failed-outbox-write gap using the repository's supported transaction pattern or durable pending-event/reconciliation pattern, documenting the choice. Email provider availability must not control whether a valid booking/signature succeeds. An event must remain recoverable after a process crash. Persist event identity from aggregate ID + transition/revision, recipient and channel. New legitimate repeat actions get new revisions; retries do not.

**NOT-03 — Channels and retries.** Retain in-app availability independent of email success; make channel retry outcomes observable. Deduplicate outbox creation, feed entries and provider requests where supported. Do not claim exactly-once email when a provider send can succeed before local acknowledgement fails. Document and test that failure window, retry limits, dead-letter handling and safe operator replay. Cancel obsolete reminders after reschedule/cancellation and regenerate from UTC instants.

**NOT-04 — Actionable UI.** Use both existing notification centres and feeds. Show readable title/time, unread badge and authorized deep link to the exact action. Preserve read state after refresh and across retries. Handle deleted/expired targets gracefully. Confirm polling/open/focus refresh works; target in-app visibility within the existing 30-second polling interval during healthy operation. Proposed email target: queued durably with the mutation and first dispatch within one minute of the worker's healthy schedule; measure before promising it.

**NOT-05 — Operational evidence.** Add structured correlation IDs and counts for queued, sent, failed, retried and dead-letter events without logging document contents or signatures. Verify worker startup/configuration and real inbox delivery in a controlled test environment. Provider acceptance is not proof a user received an email. Record deployment, database persistence, in-app appearance and controlled mailbox evidence separately.

### Verification and guards

Use existing notification service/in-app tests, booking/documents/notes notification tests, HTTP ownership tests, email renderer tests and frontend feed/centre tests as patterns. Add matrix-driven tests for new flows. Exercise queue failure, worker restart, duplicate webhook/signature retry, transient email failure, permanent failure, unavailable practitioner mapping, old reminder cancellation, multi-recipient partial success and deep-link authorization. No event for a rejected mutation or draft save; no duplicate on page reload or downloading a document.

## 8. Execution ledger and handoff rules

Implementation is authorized. Record owner, reserved paths, commit/reference, verification evidence and blocker; retain that record across handoffs. Shared files such as agreement policy, booking gates, frontend API types and composition wiring need one owner at a time.

| Work package | Depends on | Status | Owner / evidence |
|---|---|---|---|
| P0: reproduction, owner decisions, contracts | — | Code/contract complete; owner reproduction pending | Codex; static UTC/status trace, source PDF field comparison, approved policy choices |
| TZ-01–05: timezone and classification | P0 | Implemented; owner tests/data audit pending | Account preference, saved-zone UI/notifications, server-clock grouping, DST disambiguation, audit/backfill/rollback tool |
| MIN-01–05: participant and guardian | Time contract, P0 consent decisions | Implemented; owner verification pending | Booking declarations, admin-editable wording, contextual guardian evidence, readiness gates, PDFs |
| SOW-01–05: signed form | Participant/signer contract | Implemented; owner verification pending | Both source-specific forms, catalog fee, atomic signed answers, original/countersigned archives, legacy preservation |
| NOT-01–05: event coverage and delivery | Earlier event contracts | Implemented; live delivery pending | Transaction journal, recipient routing, durable retries, immutable payload/provider key, operator replay, matrix in design/in-app-notifications.md |
| QA/release: integrated verification and rollout | All above | Owner-run acceptance pending | No tests executed, browser opened, live DB modified or deployment performed; user authorized commit and push after implementation |

Complete and verify each numbered requirement before moving to the next. Include its notification event contract while implementing it, then complete the cross-cutting notification audit in phase 4. Do not commit, push, deploy or mutate production data merely because this planning document exists; follow the implementation session's authorization.

For each handoff include: objective, affected files, decisions, implemented contract, remaining tasks, commands/results, known baseline failures, migration state and next safe action. Use separate **Engineering complete** and **Externally verified** evidence; deployment status must never be inferred from local tests.

## 9. Final verification and rollout

1. Run targeted tests per phase, then the full API tests and vet, affected app tests/lint/builds and the integrated browser journey. Available commands: from `api/`, `go test ./...` and `go vet ./...`; from root, `npm run test --workspace=apps/portal`, `npm run test --workspace=apps/admin`, corresponding `lint` and `build` commands. Read `e2e/README.md` before running `npm test --prefix e2e`; its tests target a configured deployment. Record exact commands and actual outcomes rather than copying prior counts.
2. Browser journey: select and save a timezone in-app, book a paid future minor appointment in that zone, see pending status, verify payment, complete guardian consent and SOW form/signature, reload both accounts in different browser zones, review PDFs, verify both parties' notifications, reschedule and verify replacement reminders. Also execute adult and legacy unsigned-SOW paths. Check desktop, mobile and keyboard use.
3. Review API/data contracts and ownership boundaries. Search for browser-derived appointment timezone, timezone-less instant parsing, unsigned-only SOW comments and non-confirmed-as-past branches; inspect each match, do not mechanically delete valid date-only/local-display code.
4. Run migration dry-run and compatibility checks before rollout. Deploy additive API/data support before dependent UI; do not enable new signature/minor gates until all required readers/writers support them. Keep backups and an explicit rollout switch or equivalent rollback procedure. Rollback may disable new entry paths but must preserve signed evidence and outbox records.
5. Confirm persisted user timezone preferences and first-use selection, worker configuration and controlled email/in-app delivery. Monitor timezone validation failures, missing consent readiness, signing/archive failures and notification lag. Do not bulk-resend historical events on deployment.
6. Update `agent_plan.md`, `design/api-contract.md` and `design/in-app-notifications.md` with final behavior and evidence. All four requirements are complete only when acceptance tests pass and remaining external checks are clearly identified.

## 10. Owner review checklist

- [x] Timezone is user-configurable in-app; persist and consistently apply each user's selection.
- [x] Guardian name/signature in the booking flow, recommended admin-editable wording, declared guardian identity.
- [x] Checkout allowed; server-enforced session readiness gate.
- [x] Source PDF fields; fee prefilled/read-only from service pricing.
- [x] Implementation authorized; owner will run tests and browser checks.

## 11. Implementation handoff — 23 September 2026

**Code delivered; owner-run acceptance remains pending.** Scope is the API, portal and practitioner app. No browser or test suite has been run in this session. Existing unrelated workspace edits, PDF files and skill-observation notes were preserved.

### Delivered behavior

1. Explicit account timezone selection in first-use, Settings and booking/calendar controls; it saves immediately and syncs account state on focus and at 30-second intervals. A saved preference wins over browser and URL zones. Appointment instants remain UTC; `bookingTimezone` is audit context only. Availability retains its authored zone. Admin repeated DST times require choosing an explicit UTC instant; nonexistent wall times are rejected. Calendar day bounds select the earliest real instant on that civil date. Appointment lists use server time plus a monotonic clock and refresh on focus/polling.
2. Pending payment, upcoming, in-progress and past groups are distinct. Pending requests expire at the requested start on the server, with a clear expired label. Payments exposes pending booking requests even if checkout initialization never created a payment record; checkout can be resumed from that booking. Late verified charges are refunded instead of resurrecting expired requests.
3. Under-18 selection reveals guardian name, relationship, contact email, recommended wording and typed-signature controls. The guardian may sign immediately or finish in Required documents after checkout. Booking ownership stays with the authenticated account. Declaration revisions and appointment-date matching prevent reusing another participant's consent. Admin Settings edits versioned guardian wording; changing it requires new applicable guardian executions. Legacy participant status is explicitly unknown.
4. Required documents are available at `/portal/sessions/{id}/documents`. SOW forms collect and review answers and capture an acknowledged typed signature. Both supplied source PDFs were inspected: holistic uses client name/effective date/initial term/monthly fee; nurse uses client name/service start date/package/monthly fee. Participant name and service fee are read-only and validated server-side. Coaching countersignatures retain their existing document-type policy. Session tickets and socket redemption independently enforce readiness, including booking-assigned forms.
5. Portal Documents and practitioner client records show executed SOW answers, role/version, legacy unsigned labels and PDF download/archive state. Original snapshots are never rewritten. Archival retry stores separate original/countersigned PDFs using stable execution keys. Guardian PDFs include actual participant/guardian evidence and coaching practitioner blocks.
6. Covered business writes and notification events commit together in Mongo transactions. Existing inboxes/outbox are reused. Assigned recipients, per-recipient zones, exact action links, reminder invalidation, signature archival retry, immutable provider payload and safe single-ID replay are implemented. The detailed event/channel/source matrix is in `design/in-app-notifications.md`.

Recommended guardian wording (editable in practitioner Settings):

> I am the parent or legal guardian of the participant named above, who will be under 18 on the appointment date. I confirm that I have authority to consent on their behalf, have reviewed the applicable documents, and consent to the services described. I intend my signature to record this consent.

This captures a guardian declaration through the signed-in account, not externally verified guardian identity.

### Static trace and verification boundary

The original classification defect is directly addressed: `pending_payment` previously fell into the non-confirmed/past branch. The new grouping compares UTC instants and treats payment state separately. Synthetic trace for owner reproduction: select `2026-11-01T06:30:00Z` (the second New York 01:30 during fall-back), send that exact instant with `tz=America/New_York`, preserve it in Mongo/API, and display it in each saved account zone. Before payment it is pending in every zone; confirmed at start is in progress; at end it is history. `schedule-dst.test.ts` distinguishes both fall-back instants and the spring-forward gap.

Static checks used:

- API: `go build ./...` and `go test -exec /usr/bin/true ./...`. The latter **only compiles/links test binaries**; `/usr/bin/true` replaces their execution. Its `ok` output is not a passed behavioral test result.
- Portal and admin: `npx tsc --noEmit`.
- Scoped ESLint on changed TypeScript/React files, formatting and whitespace review.

Regression tests were added for account timezone persistence/rejection; lifecycle boundaries/unpaid expiry; DST fold/gap; participant validation and HTTP tampering; guardian/version/participant isolation; SOW fee/signature/answer integrity; timestamp digest precision; readiness at ticket issue and redemption; countersignature/form readiness; PDF guardian/practitioner blocks; durable retry/dedup/read state; recipient timezone; archival outages; provider acceptance before local acknowledgement; metadata privacy; and transaction rollback when journal insertion fails. Tests have **not been executed**. The Mongo atomicity test opts in only with `TERIOS_TEST_MONGODB_URI` and creates/drops its own uniquely named disposable database.

### Rollout and recovery runbook

1. Back up the database and retain the current application release. Run the read-only audit from `api/` against the intended environment: `go run ./cmd/workflowctl`. It reports counts and limited record IDs for missing zones/declarations, invalid/string timestamps, legacy unsigned SOWs, pending/dead-letter events and failed deliveries. No live counts are claimed here.
2. MongoDB must support transactions (Atlas, replica set or sharded cluster); startup now checks this. New `agreement_executions`, `workflow_events`, notification event-key and archive-key indexes are additive. Do not remove the legacy signature collection/index. During the coordinated release, pause new booking/signing entry at the edge, drain writes, deploy the API first, deploy both apps, then reopen entry. Existing old clients do not supply participant declarations and must refresh to the new app.
3. Do not infer a booking's historical zone from the account's current zone. Only if independent original provenance exists, prepare a reviewed JSON array such as `[{"bookingId":"<24-hex-id>","timezone":"America/New_York","source":"original request audit reference"}]`. Dry-run: `go run ./cmd/workflowctl --action backfill --manifest zones.json`. During a maintenance write pause, apply with `--apply --backup-out zone-backup.json`. The tool writes a new mode-0600 backup before changing anything and fills only missing/empty metadata; it never changes `startAt`/`endAt` or invents participant/signature evidence. Re-running the same manifest skips populated records. Unknown provenance remains missing.
4. Metadata rollback: `go run ./cmd/workflowctl --action rollback --manifest zone-backup.json` (dry-run), then add `--apply` after reviewing the target counts. It unsets only metadata that still matches that migration ID and proposed zone. Keep writes paused for backfill/rollback to avoid overlapping aggregate updates. An application rollback must disable booking/signing/session-entry writes until a compatible API/UI is restored; do not run an old replacement-write implementation over new records or drop evidence/outbox collections.
5. Notification event recovery: inspect the correlated failure, fix its cause, then dry-run `go run ./cmd/workflowctl --action replay-event --id <event-id>`. Add `--apply` to retry that **dead-letter** event only. For a failed email job, use `--action replay-job --id <job-id>` and inspect provider history before applying. Sent/cancelled jobs are refused; IDs, dedup keys and frozen provider payload remain unchanged. Resend's documented 24-hour idempotency window does not eliminate duplication risk after that period. No bulk replay command is provided.
6. Configure `RESEND_API_KEY`/sender, portal/dashboard base URLs and the media archive store. `NOTIFICATION_POLL_INTERVAL` defaults to 15 seconds; inbox polling remains 30 seconds. Without archive storage, signed evidence and on-demand PDFs remain available but archive delivery retries/dead-letters honestly. Measure queue lag and controlled inbox receipt separately from provider acceptance.
7. Owner acceptance: run the API/app tests and the section 9 adult/minor/legacy/DST/payment/reschedule journey, inspect generated PDFs and both recipients' persisted notifications, and run the opt-in Mongo atomicity test on a disposable replica set. Live audit results, migration writes, mailbox delivery, browser behavior, deployment and production verification are still outstanding by owner choice.

### 24 September follow-up: downloaded SOW and consultation navigation

Removed the duplicate preparation list: Required documents now stays inside dated upcoming/in-progress appointment cards, and pending payment links target the exact booking. The PDF renderer omits recognized empty SOW template prompts after rendering saved answers, preserves additional terms and stored evidence, and labels historical SOW records without answers explicitly. Signature responses expose the recorded agreement key so the portal can show the same explanation. The electronic-execution footer now uses the owner's exact replacement wording for agreements and signed SOWs.

Added PDF rendering/footer and consultation-navigation regression tests. Targeted Go test binaries and portal TypeScript compile; no behavioral tests or browser run. Inspected the owner-provided PDF from Completed forms & signed documents: it contains the version-1 nurse SOW template and a 16 September signature, with no filled answers. Its layout matches the signature-only rendering branch; the live database was not queried. Historical signature-only entries now explain the distinction from newer submissions and link to appointment documents. Also corrected the letterhead RGB color operator after Poppler reported invalid color arguments. Existing archived PDF files are not rewritten by this change.
