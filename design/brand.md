# Terios Wellness — Brand Identity Foundation

> Contract file. All values here are normative. `design/design-system.md` references these token names; frontend agents must not invent alternates.

- Brand: **Terios Wellness** (practice of a solo RN + wellness coach; video-first, international clients, Ghana-based).
- Two surfaces: `apps/web` (public marketing + client portal, **light-first**) and `apps/admin` (practice dashboard, supports light + dark).
- Wordmark: "Terios" set in Figtree, weight 600, letter-spacing -0.03em; "Wellness" may be dropped in compact contexts. Never letterspace the wordmark wide, never all-caps it.

---

## 1. Brand Essence

**One line:** Clinical calm — the trust of a nurse, the exhale of a spa.

Terios sits between a medical practice and a wellness retreat. It must never read as a hospital (cold, fluorescent, bureaucratic) and never as a beauty influencer (playful, pastel-candy, unserious). Every screen should lower the viewer's shoulders.

**Positioning pillars**

1. **Credentialed** — a registered nurse runs this. Precision, confidentiality, structure.
2. **Calm** — generous whitespace, low contrast noise, slow motion, no urgency tricks.
3. **Warm** — botanical and earthen, human, never sterile.
4. **Premium** — editorial typography, restrained color, exact alignment. Spa-grade finish, not SaaS-grade.

**Personality sliders**

| Axis | Position |
|---|---|
| Clinical ↔ Nurturing | 65% nurturing, but the 35% clinical is load-bearing |
| Serene ↔ Energetic | 85% serene |
| Minimal ↔ Rich | 70% minimal; richness comes from type and photography, not ornament |
| Formal ↔ Personal | 60% personal (first-person voice, founder-led) |

---

## 2. Voice & Tone

Voice is constant; tone flexes by context. Voice: **a calm, competent practitioner speaking to one person** — second person ("you"), short sentences, no jargon, no exclamation marks except in genuine celebration (booking confirmed is not a celebration; a completed program milestone is).

**Rules**

- Sentence case everywhere, including buttons and headings. Never Title Case, never ALL CAPS except micro-labels (badges, eyebrow text) at 11px with +0.08em tracking.
- Buttons are verbs, 1–3 words: "Book a session", "Save notes", "Join call".
- Errors say what happened and what to do next. No blame, no "Oops", no "Something went wrong" alone.
- No false urgency ("Only 2 spots left!"), no dark patterns, no emoji in UI copy. One botanical glyph (❦) is reserved for the marketing footer only — nowhere else.
- Numbers and dates: "Tue, Aug 12 · 3:30 PM (GMT)" — always show timezone on scheduling surfaces.

**Do / Don't (UI copy)**

| Context | Do | Don't |
|---|---|---|
| Primary CTA | "Book your first session" | "Get Started Now!!" |
| Empty state (portal) | "No sessions yet. When you book one, it will appear here." | "No data found." |
| Form error | "Enter a valid email address, e.g. you@example.com" | "Invalid input" / native browser bubble |
| Payment failure | "Your card was declined. Try another card or contact your bank." | "Transaction error 5021" |
| Session reminder | "Your session with Akosua is tomorrow at 3:30 PM (GMT)." | "REMINDER: APPOINTMENT IN 24 HOURS" |
| Success toast | "Session booked. We've emailed your video link." | "Success!" |
| Destructive confirm | "Cancel this session? Your credit will be refunded within 5 days." | "Are you sure?" |
| Loading | "Preparing your room…" | "Loading…" (bare spinner with no words is fine; bare "Loading…" is not) |
| 404 | "This page has moved or no longer exists." + link home | "404 Not Found" |

Tone shift by surface: **marketing** = aspirational, slower, longer sentences allowed. **Client portal** = warm but efficient. **Admin** = terse, precise, neutral (the practitioner is the user here, not the audience: "3 bookings today", not "You have 3 lovely bookings today!").

---

## 3. Color

Light theme is the brand default. The palette has three families: **Eucalyptus** (primary green — calm, clinical-botanical), **Clay** (warm accent — human, earthen; used sparingly), **Sand** (warm neutral surfaces — paper, not gray). Semantic colors are tuned to sit inside these families so nothing on screen ever clashes.

### 3.1 Eucalyptus (primary)

| Token | Hex | Use |
|---|---|---|
| `eucalyptus-50` | `#F2F6F3` | tinted surfaces, hover fills on sand |
| `eucalyptus-100` | `#E4EEE8` | selected rows, subtle chips |
| `eucalyptus-200` | `#C6DCCF` | borders on tinted surfaces |
| `eucalyptus-300` | `#9DC3AE` | disabled fills, decorative |
| `eucalyptus-400` | `#6FA389` | hover on dark text links |
| `eucalyptus-500` | `#4E8469` | primary **hover/pressed** |
| `eucalyptus-600` | `#3B6B54` | **primary** (buttons, links, active states) |
| `eucalyptus-700` | `#2F5744` | primary pressed text on light |
| `eucalyptus-800` | `#264335` | dark headings accent |
| `eucalyptus-900` | `#1C3328` | dark-theme surface tint |

### 3.2 Clay (accent — max ~5% of any screen)

| Token | Hex | Use |
|---|---|---|
| `clay-50` | `#FBF3EE` | highlight panels, testimonial bg |
| `clay-100` | `#F5E2D6` | soft badge bg |
| `clay-300` | `#DEA684` | decorative |
| `clay-500` | `#C07A56` | **accent** (premium tags, star ratings, highlights) |
| `clay-600` | `#A86443` | accent hover |
| `clay-700` | `#874F34` | accent text on light |

### 3.3 Sand (warm neutrals)

| Token | Hex | Use |
|---|---|---|
| `sand-0` | `#FDFCFA` | elevated surface (cards, modals) |
| `sand-50` | `#FAF7F2` | **page background** (`surface`) |
| `sand-100` | `#F3EEE5` | inset wells, sidebar bg |
| `sand-200` | `#E7DFD0` | **borders/dividers** (`border`) |
| `sand-300` | `#D3C8B2` | strong borders, input borders |
| `sand-400` | `#AE9F82` | placeholder text, disabled text |
| `sand-500` | `#8A7B62` | **muted text** (`ink-muted`) |
| `sand-600` | `#6B5F4B` | secondary text |
| `sand-700` | `#524838` | body-strong |
| `sand-800` | `#38311F` | near-ink |
| `sand-900` | `#211D12` | — (superseded by ink, below) |

### 3.4 Semantic roles (light theme — normative)

| Token name | Value | Role |
|---|---|---|
| `primary` | `eucalyptus-600 #3B6B54` | primary actions, links, focus ring core |
| `primary-hover` | `eucalyptus-500 #4E8469` | hover |
| `primary-active` | `eucalyptus-700 #2F5744` | pressed |
| `on-primary` | `#FDFCFA` | text on primary (contrast 7.1:1 ✓) |
| `accent` | `clay-500 #C07A56` | sparse highlights, rating stars, "signature" details |
| `surface` | `sand-50 #FAF7F2` | page background |
| `surface-raised` | `sand-0 #FDFCFA` | cards, popovers, modals |
| `surface-sunken` | `sand-100 #F3EEE5` | input wells, sidebars, table stripes |
| `ink` | `#1F2922` | primary text (deep forest, not black) |
| `ink-muted` | `#6E6350` | secondary text (4.6:1 on surface ✓; darker than `sand-500`, which is decorative only) |
| `ink-faint` | `sand-400 #AE9F82` | placeholders, disabled text only (never body) |
| `border` | `sand-200 #E7DFD0` | hairlines, card borders |
| `border-strong` | `sand-300 #D3C8B2` | input borders, separators |
| `success` | `#3F7A5C` | confirmations (bg: `eucalyptus-100`, text: `#2A5A42`) |
| `warning` | `#9A6B27` | cautions (bg: `#F7EDDA`, text: `#7A5420`) |
| `danger` | `#A93F3F` | destructive actions (bg: `#F7E7E4`, text: `#8A3232`, hover: `#8F3535`) |
| `info` | `#4A6E7E` | neutral notices (bg: `#E8EFF1`, text: `#3A5866`) |
| `focus-ring` | `eucalyptus-600` at 40% alpha | 2px ring + 2px offset, see §6 |

**Usage discipline**

- One primary action per view. Clay appears at most twice per screen (e.g. a star rating + one badge) — it is a signature, not a theme.
- No gradients anywhere in product UI. Marketing may use a single static duotone photo treatment (eucalyptus-900 multiply over photography at 35%), never CSS color gradients.
- Borders, not shadows, define structure on the page background; shadows are reserved for raised surfaces (popovers, modals, toasts).
- **Forbidden:** pure black `#000`, pure white page bg, cool grays (blue-channel grays), SaaS indigo/violet/blue-600, default Tailwind palette names in code, neon anything.

### 3.5 Dark theme (admin app only, optional for portal later)

Dark is a **deep botanical night**, not inverted light and not blue-black:

| Token | Value |
|---|---|
| `surface` | `#141A16` (deep forest charcoal) |
| `surface-raised` | `#1C241E` |
| `surface-sunken` | `#101512` |
| `ink` | `#EDE9DF` (warm off-white) |
| `ink-muted` | `#9A9482` |
| `ink-faint` | `#6B6657` |
| `border` | `#2A332C` |
| `border-strong` | `#3A453C` |
| `primary` | `eucalyptus-400 #6FA389` (lightened for contrast) |
| `primary-hover` | `#82B298` |
| `on-primary` | `#12241B` |
| `accent` | `clay-300 #DEA684` |
| `success` `#6FA389` · `warning` `#D9A45B` · `danger` `#D97B6F` · `info` `#8AA8B5` | (with 12%-alpha tinted bg variants) |

Admin defaults to system preference, toggle persists per-user. Customer app ships light-only in v1; do not build a half-dark portal.

---

## 4. Typography

Two open-license fonts, both on Google Fonts + Fontsource:

- **Display sans — Figtree** (OFL). Humanist-geometric, warm, and precise. Used for marketing headlines, page titles, pull quotes, empty-state headlines, and numeric displays.
- **Body sans — Outfit** (OFL). Open and contemporary without losing warmth. Used for body copy, forms, tables, navigation, and buttons. Weights used: 400, 500, 600.

Loading: Fontsource self-hosted (`@fontsource-variable/outfit`, `@fontsource/figtree`), `font-display: swap`, subsets `latin` + `latin-ext` (international clients). Both fall back to `system-ui, -apple-system, 'Segoe UI', sans-serif`.

### Type scale (1.25 ratio-ish, hand-tuned; px at 16px root)

| Token | Font / weight | Size | Line-height | Letter-spacing | Use |
|---|---|---|---|---|---|
| `display-xl` | Figtree 600 | 64/56px (4/3.5rem) | 1.05 | -0.03em | marketing hero only |
| `display-lg` | Figtree 600 | 48/40px | 1.1 | -0.025em | marketing section heads |
| `display-md` | Figtree 600 | 36/32px | 1.15 | -0.02em | page titles (portal), empty states |
| `display-sm` | Figtree 600 | 28/24px | 1.2 | -0.015em | card feature titles, modal titles |
| `heading-lg` | Figtree 600 | 22px | 1.3 | -0.005em | section heads inside app |
| `heading-md` | Figtree 600 | 18px | 1.35 | 0 | card headers, drawer titles |
| `heading-sm` | Figtree 600 | 16px | 1.4 | 0 | table headers row, list group heads |
| `body-lg` | Figtree 400 | 18px | 1.6 | 0 | marketing lead paragraphs |
| `body-md` | Figtree 400 | 16px | 1.6 | 0 | default body, form inputs |
| `body-sm` | Figtree 400 | 14px | 1.55 | 0 | dense app text, table cells, hints |
| `label` | Figtree 500 | 14px | 1.4 | +0.005em | form labels, button text (500/600) |
| `caption` | Figtree 500 | 13px | 1.45 | +0.01em | helper text, timestamps, table meta |
| `micro` | Figtree 600 | 11px | 1.3 | +0.08em, uppercase | badges, eyebrows, overlines |

Notes: body text max-width 68ch (`measure`); marketing body 60–68ch, never full-width. Hyphenation off, `text-wrap: pretty` on display/heading, `text-wrap: balance` on hero. Tabular numerals (`font-feature-settings: "tnum"`) in tables, calendars, pricing, and time displays.

---

## 5. Spacing, Radius, Shadow

**Spacing** — 4px base unit. Scale (px): `0, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80, 96, 128` → tokens `space-0 … space-14`. Layout rhythm: app screens use `space-6` (24px) gutters and `space-8/10` section gaps; marketing sections use `space-20` (80px) vertical padding desktop, `space-12` mobile.

**Radius** (`radius-*`):

| Token | Value | Use |
|---|---|---|
| `radius-sm` | 6px | chips, badges, small inputs |
| `radius-md` | 10px | buttons, inputs, selects, tooltips |
| `radius-lg` | 16px | cards, modals, drawers, calendar cells |
| `radius-xl` | 24px | marketing feature cards, image frames |
| `radius-full` | 9999px | pills, avatars, switches, dots |

Organic-but-controlled: no fully-round cards, no 0px sharpness. Switches and avatars are the only `radius-full` shapes.

**Shadow** (`shadow-*`) — warm-tinted (sand/eucalyptus hue), low elevation, never black-heavy:

| Token | Value | Use |
|---|---|---|
| `shadow-xs` | `0 1px 2px rgba(31,41,34,0.05)` | input focus stack, subtle lift |
| `shadow-sm` | `0 1px 2px rgba(31,41,34,0.06), 0 2px 8px rgba(31,41,34,0.05)` | raised cards on hover |
| `shadow-md` | `0 2px 4px rgba(31,41,34,0.06), 0 8px 24px rgba(31,41,34,0.08)` | dropdowns, popovers, sticky nav |
| `shadow-lg` | `0 4px 8px rgba(31,41,34,0.06), 0 16px 48px rgba(31,41,34,0.12)` | modals, drawers, toasts |
| `shadow-none` | none | default state of cards (border-defined) |

Dark theme: shadows replaced by 1px `border` + slightly lighter `surface-raised`; do not reuse light shadows.

---

## 6. Motion

Calm, physical, purposeful. Elements move like paper and breath — decelerating, never bouncing.

| Token | Value | Use |
|---|---|---|
| `duration-instant` | 100ms | hover color/underline |
| `duration-fast` | 150ms | button state, icon swaps |
| `duration-base` | 250ms | dropdowns, tooltips, tabs, accordion |
| `duration-slow` | 350ms | modals, drawers, toasts, page elements |
| `duration-page` | 400ms | route/view transitions (max allowed) |
| `ease-out` | `cubic-bezier(0.22, 1, 0.36, 1)` | entrances, expansions (default) |
| `ease-in` | `cubic-bezier(0.5, 0, 0.75, 0)` | exits only |
| `ease-in-out` | `cubic-bezier(0.45, 0, 0.25, 1)` | position/size changes of persistent elements |

Rules: entrances fade + translate 8–12px (or scale 0.98→1 for overlays); exits fade only, 60–75% of entrance duration, `ease-in`. **No spring/bounce easing anywhere** (no `cubic-bezier` with overshoot >1). Skeletons shimmer with a 1600ms linear sweep. `prefers-reduced-motion`: replace all transforms with opacity-only ≤150ms; TimeSlotPicker and Calendar animate height via opacity+crossfade instead. Video-room controls never animate while a call is marked unstable.

**Focus ring (global):** `outline: 2px solid color-mix(in srgb, primary 40%, transparent); outline-offset: 2px; border-radius` matches element radius. Never `outline: none` without replacement; never rely on browser default ring. Focus-visible only (not on mouse click).

---

## 7. Imagery & Iconography

**Photography:** warm natural light, real textures (linen, clay, botanicals, wood), skin in daylight, Ghanaian context welcome and preferred over stock-y spa tropes (no hot stones on backs, no cucumber eyes). Treatment: slightly desaturated, warm white balance, soft contrast; optional eucalyptus-900 multiply duotone at 35% for hero overlays with text. Aspect ratios: marketing 4:5 portraits, 3:2 features, 16:9 heroes; avatars always square crops. Radius `radius-xl` on feature imagery, `radius-full` on avatars.

**Illustration:** avoid in v1. If needed later: single-weight botanical line drawings in `ink-muted`, never filled cartoons.

**Iconography:** one icon family only — **Lucide** (ISC license, 1.5px stroke at 24px grid, rounded joins — matches the brand's softness and Figtree's geometry). Sizes: 16px (`body-sm`/buttons), 20px (default UI), 24px (feature/empty states). Stroke never modified; no mixing with emoji or other icon sets. Status colors applied via semantic tokens, not icon variants.

**Logo/mark:** wordmark-only in v1 (Figtree "Terios"). A small eucalyptus-leaf glyph may accompany it at ≥24px height, drawn as a single 1.5px Lucide-matched stroke; never a filled clip-art leaf.
