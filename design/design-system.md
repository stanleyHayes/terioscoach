# Terios Wellness — Design System Spec

> Contract file for all frontend agents. Tokens are defined in `design/brand.md` and referenced here by name (`primary`, `surface-raised`, `radius-md`, `duration-base`, …). Anything interactive listed below is a **custom-built component** — native UI elements are forbidden platform-wide.

## 0. Hard rules (applies to every component)

- **No native UI elements.** Forbidden: unstyled `<select>`, `<input type="date|time|file|color|range">`, `<dialog>` chrome, native validation bubbles (`reportValidity()`), native scrollbars, default focus outlines, native tooltips (`title` attr). Custom implementations are specified below. `<input type="text|email|password|tel|number">` may be used as the *base* of TextInput but must be fully restyled per §3.
- **Every interactive element** has these states styled: default, hover, focus-visible, active, disabled. Where listed: loading, error, selected, readonly.
- **Keyboard + ARIA** are part of "done", not polish. Each component lists its pattern (mostly WAI-APG). Focus is always visible per brand §6 ring.
- **Motion** uses only brand tokens; entrances ≤ `duration-slow`, no bounce. `prefers-reduced-motion` honored.
- **Theming:** all colors via CSS custom properties (§Token contract). No raw hex in component code.
- **Density:** one density platform-wide (no compact mode). Touch targets ≥ 40×40px even when visual size is smaller.
- **Text:** `label` token for buttons/labels; `body-md` for inputs; `caption`/`micro` for meta. No text smaller than 11px.

---

## 1. Token contract (both Next.js apps implement identically)

CSS custom properties on `:root` (light) and `[data-theme="dark"]` (admin), then mapped into Tailwind v4 `@theme`. File: `src/app/globals.css` in each app, imported from a shared source of truth (copy verbatim until a shared package exists — values must not drift; this file is the arbiter).

```css
:root {
  /* color */
  --primary: #3B6B54; --primary-hover: #4E8469; --primary-active: #2F5744; --on-primary: #FDFCFA;
  --accent: #C07A56; --accent-hover: #A86443;
  --surface: #FAF7F2; --surface-raised: #FDFCFA; --surface-sunken: #F3EEE5;
  --ink: #1F2922; --ink-muted: #6E6350; --ink-faint: #AE9F82;
  --border: #E7DFD0; --border-strong: #D3C8B2;
  --success: #3F7A5C; --success-bg: #E4EEE8; --success-ink: #2A5A42;
  --warning: #9A6B27; --warning-bg: #F7EDDA; --warning-ink: #7A5420;
  --danger: #A93F3F; --danger-hover: #8F3535; --danger-bg: #F7E7E4; --danger-ink: #8A3232;
  --info: #4A6E7E; --info-bg: #E8EFF1; --info-ink: #3A5866;
  --focus-ring: color-mix(in srgb, var(--primary) 40%, transparent);
  --overlay: rgba(28, 51, 40, 0.45);           /* modal/drawer scrim, eucalyptus-900 @45% */
  /* radius */
  --radius-sm: 6px; --radius-md: 10px; --radius-lg: 16px; --radius-xl: 24px; --radius-full: 9999px;
  /* shadow */
  --shadow-xs: 0 1px 2px rgba(31,41,34,.05);
  --shadow-sm: 0 1px 2px rgba(31,41,34,.06), 0 2px 8px rgba(31,41,34,.05);
  --shadow-md: 0 2px 4px rgba(31,41,34,.06), 0 8px 24px rgba(31,41,34,.08);
  --shadow-lg: 0 4px 8px rgba(31,41,34,.06), 0 16px 48px rgba(31,41,34,.12);
  /* motion */
  --duration-instant: 100ms; --duration-fast: 150ms; --duration-base: 250ms;
  --duration-slow: 350ms; --duration-page: 400ms;
  --ease-out: cubic-bezier(.22,1,.36,1); --ease-in: cubic-bezier(.5,0,.75,0);
  --ease-in-out: cubic-bezier(.45,0,.25,1);
  /* space (4px base) */
  --space-1: 4px;  --space-2: 8px;  --space-3: 12px; --space-4: 16px; --space-5: 20px;
  --space-6: 24px; --space-8: 32px; --space-10: 40px; --space-12: 48px; --space-16: 64px;
  --space-20: 80px; --space-24: 96px; --space-32: 128px;
  /* type */
  --font-display: "Figtree", system-ui, sans-serif;
  --font-sans: "Outfit Variable", system-ui, sans-serif;
  /* z-index scale */
  --z-sticky: 20; --z-dropdown: 40; --z-overlay: 60; --z-modal: 70; --z-toast: 80; --z-tooltip: 90;
}
[data-theme="dark"] { /* admin only — full values in brand.md §3.5 */
  --primary: #6FA389; --primary-hover: #82B298; --primary-active: #4E8469; --on-primary: #12241B;
  --accent: #DEA684; --surface: #141A16; --surface-raised: #1C241E; --surface-sunken: #101512;
  --ink: #EDE9DF; --ink-muted: #9A9482; --ink-faint: #6B6657;
  --border: #2A332C; --border-strong: #3A453C; --focus-ring: color-mix(in srgb, var(--primary) 55%, transparent);
  --overlay: rgba(0,0,0,.55); --shadow-xs: none; --shadow-sm: none; --shadow-md: none; --shadow-lg: none;
}
@theme inline {
  --color-primary: var(--primary); --color-primary-hover: var(--primary-hover); /* …all colors… */
  --radius-md: var(--radius-md); --shadow-md: var(--shadow-md); --font-sans: var(--font-sans);
  /* full mechanical 1:1 mapping of every var above; no Tailwind default palette usage */
}
```

Tailwind usage: `bg-surface-raised text-ink border-border rounded-lg shadow-md` etc. If a needed token is missing, **add it here first** — do not hardcode.

---

## 2. Layout grids

**Marketing (apps/web public)**

- Max content width 1200px, page gutters 24px mobile / 48px desktop.
- 12-col grid, 24px gutters ≥1024px; 4-col, 16px gutters <768px.
- Section vertical padding: `space-20` desktop, `space-12` mobile. Hero: min-height 88vh, content vertically centered.
- Prose/legal pages: 720px max column, centered.
- Breakpoints: `sm 640 / md 768 / lg 1024 / xl 1280` (Tailwind defaults; do not add more).

**Client portal (apps/web authed)**

- Top Nav (§27) + content max 960px, gutters `space-6`. Two-column patterns allowed: 2fr/1fr (main/detail) ≥1024px, stacked below.

**Admin dashboard (apps/admin)**

- Fixed sidebar 264px (collapsible to 72px icon rail), content area fluid to max 1440px, gutters `space-6`, page padding `space-8` top.
- Data screens: header row (title + actions) → filter bar → table/card region. No marketing-style hero spacing in admin.

**Scrollbar (global, custom):** `::-webkit-scrollbar` width 10px; track transparent; thumb `border-strong`, `radius-full`, 2px transparent border via `background-clip: padding-box`; hover thumb `ink-faint`. Firefox: `scrollbar-width: thin; scrollbar-color: var(--border-strong) transparent`. Never hide scrollbars where scroll exists.

---

## 3. Components

Conventions below: action sizes are `sm` (36px tall) / `md` (44px) / `lg` (52px) unless a dense data control is explicitly stated; default `md`. State changes animate `duration-fast ease-out` unless noted. Disabled = `opacity .5, cursor: not-allowed, pointer-events: none` (except form fields, which stay focusable-readonly per ARIA).

### 3.1 Button

Anatomy: tactile pill container + optional leading icon (16px) + label (`label`, weight 600) + optional trailing icon + the 5px clay care-point on primary actions. Height 44px md, padding `0 space-5`, `radius-full`, gap 8px. The 1px inset highlight and care-point identify a Terios action without adding decorative icons.

| Variant | Default | Hover | Active | Focus |
|---|---|---|---|---|
| primary | bg `primary`, text `on-primary`, `shadow-xs` | bg `primary-hover` | bg `primary-active`, translateY(0) (no press-shift) | brand focus ring |
| secondary | bg transparent, border 1px `border-strong`, text `ink` | bg `surface-sunken`, border `ink-faint` | bg `eucalyptus-100` | ring |
| ghost | bg transparent, text `primary` | bg `eucalyptus-50` | bg `eucalyptus-100` | ring |
| danger | bg `danger`, text `on-primary` | bg `danger-hover` | darken 5% | ring (danger 40%) |

States: hover lifts 2px only on fine pointers; active scales to .975 for immediate acknowledgement. **Loading** — width locked, label replaced by 16px spinner (Lucide `LoaderCircle`, 900ms linear rotate, `currentColor`) + original label kept for screen readers (`aria-busy="true"`, disabled). **Disabled** per convention. **Full-width** option for mobile CTAs. Sizes: sm 36px/14px text, md 44px/14px, lg 52px/16px. Keyboard: native `<button>` semantics; `Enter`/`Space` activate. One primary button per view region.

### 3.2 IconButton

Square button, icon 20px centered. Sizes sm 32/md 40/lg 48. Variants: ghost (default; hover `surface-sunken`), outline (border `border-strong`, hover `surface-sunken`), filled (bg `primary`, icon `on-primary`). **Mandatory `aria-label`**; Tooltip (§15) on hover/focus for non-obvious icons, 300ms delay. Used in tables rows, VideoRoom chrome, modal close.

### 3.3 TextInput

Anatomy: Field wrapper (§30) + control. Control: height 40px md (sm 32, lg 48), bg `surface-raised` (portal) / `surface-sunken` optional on raised cards, border 1px `border-strong`, `radius-md`, padding `0 space-3`, text `body-md ink`, caret `primary`. Placeholder `ink-faint`, never as label substitute.

- hover: border `ink-faint`. focus: border `primary` + focus ring. filled: unchanged (no bg shift).
- **error**: border `danger`, ring danger 40% on focus; error text below (§30); `aria-invalid="true"`.
- **disabled**: bg `surface-sunken`, text `ink-faint`, border `border`.
- **readonly**: bg transparent, border `border`, no ring.
- Optional leading icon 16px `ink-faint` (padding-left 36px), trailing slot for clear button (IconButton sm ghost, appears when non-empty) or password visibility toggle (`Eye`/`EyeOff`).
- Never `type=date/time/file` (see DatePicker, FileUpload). `type=number` only with `inputmode="numeric"` and custom steppers if needed (hide native spinners: `appearance: none`).

### 3.4 TextArea

TextInput rules; min-height 96px, padding `space-3`, resize: vertical only (custom handle not required — browser resize grip restyled: 16px Lucide `Grip` glyph bottom-right in `ink-faint`). Optional character counter (`caption ink-faint`, bottom-right inside padding, e.g. "120/500"); at 90% counter turns `warning`, at max hard-stop with `aria-live` note. Auto-grow variant for chat-like inputs: min 40px, max 200px, then custom scrollbar.

### 3.5 Select (custom listbox)

**Never native `<select>`.** Trigger: styled exactly as TextInput with label text `body-md ink` (placeholder `ink-faint`) + trailing `ChevronDown` 16px (rotates 180° on open, `duration-base`). Popup: `surface-raised`, border `border`, `radius-lg`, `shadow-md`, padding 4px, max-height 280px + custom scrollbar, width = trigger width, opens downward 4px offset (flips up on collision).

- Option: height 36px, padding `0 space-3`, `radius-sm`, `body-sm`. Hover: bg `eucalyptus-50`. Selected: `Check` 16px `primary` trailing + text `ink` weight 500. Focused (active descendant): bg `eucalyptus-100`.
- Keyboard/ARIA: ARIA listbox pattern — trigger `role="combobox" aria-expanded aria-controls`, popup `role="listbox"`, options `role="option" aria-selected`. Typeahead, ↑/↓ navigate, `Enter`/`Space` select, `Esc` close & refocus, `Home`/`End`.
- Entrance: fade + translateY(-4px), `duration-base ease-out`; exit fade 150ms.

### 3.6 Combobox / Search

TextInput with `role="combobox"` + popup `role="listbox"` of filtered results. Option rows 44px: leading icon/avatar 20px, primary text `body-sm`, optional secondary `caption ink-faint`. Highlight matched substring weight 600 (not color). Group headers: `micro ink-faint`, 8px top padding, sticky within popup. "No results" row: `body-sm ink-muted` + optional action row ("Invite client"). Debounce 200ms; loading shows 3 Skeleton text rows in popup. Keyboard: combobox pattern; `aria-activedescendant`; `Esc` clears then closes. Global search (admin) opens full-width 480px centered modal-style palette (`⌘K`), `surface-raised radius-lg shadow-lg`, scrim `overlay`.

### 3.7 Checkbox

Custom: 18px box (sm 16px), `radius-sm`, border 1.5px `border-strong`, bg `surface-raised`. Hover: border `primary`. Checked: bg `primary`, border `primary`, white `Check` 14px drawn with 150ms stroke-dashoffset animation. Indeterminate: white 8×2px dash. Focus ring on box. Disabled: border `border`, bg `surface-sunken`, checked fill `eucalyptus-300`. Label `body-sm ink` 8px right; whole label clickable. Error (group-level): box border `danger`. Markup: `<button role="checkbox" aria-checked="true|false|mixed">` or native input visually-hidden — either acceptable; no visible native box. Groups use `role="group" aria-labelledby`.

### 3.8 Radio

20px circle, border 1.5px `border-strong`. Selected: border `primary` + inner dot 10px `primary`, scale 0→1 `duration-fast ease-out`. Hover/focus/disabled parallel Checkbox. `role="radiogroup"` + native hidden inputs or `role="radio"`; arrow-key navigation between options, `Space` select. RadioCard variant (plan/service selection): Card (§20) with `aria-checked`, selected gets border `primary` 1.5px + `eucalyptus-50` bg + check chip top-right.

### 3.9 Switch

Track 40×22px `radius-full`, thumb 18px white `radius-full` `shadow-xs`, 2px inset. Off: track `sand-300`; hover `ink-faint`. On: track `primary`; thumb translateX(18px) `duration-base ease-out`. Disabled: opacity .5. Loading: thumb replaced by 12px spinner in track color. Label `body-sm`, switch right of label in settings rows, left in forms. `role="switch" aria-checked`, `Space`/`Enter` toggles. Never use for instantaneous destructive actions.

### 3.10 DatePicker

**Never native date input.** Trigger: TextInput-styled, read-only display "Tue, Aug 12, 2026", leading `Calendar` icon 16px. Popup: Calendar month grid (§3.12, month mode) 304px wide, `surface-raised radius-lg shadow-md border`. Footer row: "Clear" (ghost sm) / "Today" (ghost sm). Optional min/max, disabled-date predicate; disabled days `ink-faint`, no hover, `aria-disabled`.

- Selected day: bg `primary` text `on-primary radius-full` (36px cell). Today: 1px `primary` ring inside cell. Hover: `eucalyptus-100`. Focused: focus ring on cell.
- Keyboard: full ARIA grid pattern — arrows move day, `PgUp/PgDn` month, `Shift+PgUp/PgDn` year, `Home/End` week edges, `Enter` select & close, `Esc` close.
- Range variant: two triggers ("Start" / "End"), one popup; range fill `eucalyptus-100` between endpoints (endpoint cells keep `primary`).

### 3.11 TimeSlotPicker (booking availability)

Purpose-built for session booking. Vertical list or 3-col grid of time chips inside a bordered well (`surface-sunken radius-lg`, padding `space-3`), grouped by day-part with `micro ink-faint` headers ("Morning", "Afternoon", "Evening").

- Chip: height 36px, min-width 88px, `radius-md`, border `border-strong`, bg `surface-raised`, text `body-sm` with `tnum`, e.g. "3:30 PM".
- Hover: border `primary`, bg `eucalyptus-50`. Selected: bg `primary`, text `on-primary`, border `primary`. Focus: ring. **Unavailable**: strikethrough off, instead `ink-faint` text, `surface-sunken` bg, `cursor: not-allowed`, `aria-disabled` — remove from tab order. **Held by another client** (race condition): chip shakes 2px horizontal 150ms once, switches to unavailable, inline `caption danger-ink` "Just booked — pick another time".
- Timezone chip pinned above grid: `Globe` icon + "Times in GMT" (caption); tapping cycles client-local ↔ practice time.
- `role="listbox"`, chips `role="option" aria-selected`. Loading: 6 Skeleton chips. Legend below: dot `success` "Available" / dot `border-strong` "Unavailable".

### 3.12 Calendar (month & week views, scheduling)

Shared chrome: header row — `display-sm` month label (customer) / `heading-lg` (admin), nav IconButtons (`ChevronLeft/Right`, ghost), "Today" ghost sm, view switcher (Tabs sm, §17). Grid borders `border` hairlines, header row weekday `micro ink-faint`.

- **Month cell**: min-height 96px (admin) / 72px (portal), day number `caption` top-left (today: `primary` weight 600 + dot). Event chip: height 22px, `radius-sm`, padding `0 6px`, `micro` (11px, no uppercase), truncated; color by status — booked `success-bg`/`success-ink`, pending `warning-bg`/`warning-ink`, cancelled `danger-bg`/`danger-ink` strikethrough, blocked `surface-sunken`/`ink-muted`. "+2 more" chip ghost. Outside-month cells: bg `surface-sunken` 50%, numbers `ink-faint`.
- **Week view**: columns = days, hour rows 48px, now-line: 1px `accent` + 8px `accent` dot at left edge. Events: absolute blocks, `radius-sm`, left border 3px status color, bg status-bg at 70%, text status-ink `caption`; 15-min minimum visual height 24px. Drag to reschedule (admin): ghost of event at 50% opacity follows pointer; snap 15min; drop animates 200ms `ease-out` into slot; invalid drop returns with 200ms ease-in-out.
- Keyboard: grid pattern per §3.10; events reachable via `Enter` on cell then ↑/↓; `aria-label` on each event chip includes time + client + status.

### 3.13 FileUpload (dropzone)

**Never native file input chrome.** Zone: dashed 1.5px `border-strong`, `radius-lg`, min-height 160px, bg `surface-sunken` 50%, centered content: `UploadCloud` 24px `ink-muted`, `body-sm` "Drag files here or" + underlined `primary` "browse" (real visually-hidden `<input type=file>` triggered by it). Hover/focus-within: border `primary`, bg `eucalyptus-50`.

- Drag-over (valid): border solid `primary`, bg `eucalyptus-100`, `Check`-badge appears. Drag-over (invalid type): border `danger`, `body-sm danger-ink` "PDF, JPG or PNG only".
- File row (after add): 48px, `surface-raised radius-md border`, file-type icon 20px, name `body-sm` (truncated), size `caption ink-faint`, progress: 3px `radius-full` bar, track `border`, fill `primary` (indeterminate: 30% bar sliding 1200ms linear); error state fill `danger` + retry IconButton. Remove: IconButton ghost `X`.
- Constraints surfaced upfront: "Up to 3 files · 10 MB each · PDF, JPG, PNG" as `caption ink-faint` under zone. `aria-live="polite"` announces uploads; list is `role="list"`.

### 3.14 Modal

Built on native `<dialog>` element **only as an a11y primitive** (for focus trapping/`::backdrop`) with all chrome custom. Scrim: `overlay`, 250ms fade. Panel: `surface-raised radius-xl shadow-lg`, width 480px default (sm 400 / lg 640 / form 560), max-width calc(100vw - 32px), max-height 85vh with internal custom scroll, padding `space-6`. Entrance: fade + scale .96→1 + translateY(8px→0), `duration-slow ease-out`; exit fade+scale .98, 200ms `ease-in`.

- Header: `display-sm` title, optional `body-sm ink-muted` description, close IconButton top-right. Body `body-md`. Footer: right-aligned actions, gap `space-3`, primary last; destructive confirm uses danger Button + explicit consequence copy (voice rules).
- Focus: trapped; initial focus on first field or primary action (destructive: focus cancel); `Esc` closes unless a form is dirty (then confirm). `aria-modal`, `aria-labelledby`. Mobile <640px: becomes bottom sheet — full-width, top `radius-xl`, slide-up 350ms, 48px drag handle area (decorative 32×4px `border-strong radius-full` pill).

### 3.15 Drawer

Right-anchored panel (admin detail views, filters): width 420px (filters 360px), full height, `surface-raised`, left border `border`, `shadow-lg`, header like Modal with `heading-md`. Entrance translateX(100%→0) `duration-slow ease-out`; scrim `overlay` (click closes). Focus trapped, `Esc` closes. Customer portal uses Drawer for mobile nav (from left, 288px, `surface-sunken` bg).

### 3.16 Toast

Region fixed bottom-right (mobile: bottom-center, full-width minus 32px), `z-toast`, stack gap `space-2`, max 3 visible. Toast: min-width 320px max 420px, `surface-raised`, border `border`, `radius-lg`, `shadow-lg`, padding `space-3 space-4`, 3px left accent bar in status color. Anatomy: status icon 18px (`CheckCircle2`, `AlertTriangle`, `XCircle`, `Info` in status ink) + message `body-sm ink` + optional action (ghost sm) + close `X` 16px `ink-faint`.

- Entrance: translateY(12px)+fade, `duration-slow ease-out`; exit fade+collapse height, 250ms. Auto-dismiss: success/info 5s, warning 8s, error persists until dismissed; hover pauses timer (remaining-time shown by accent bar? no — keep simple, pause silently).
- `role="status"` (`role="alert"` for error), `aria-live` polite/assertive. Copy per voice rules ("Session booked. We've emailed your video link.").

### 3.17 Tooltip

Custom only (no `title`). 220ms delay (0ms if another tooltip open). Box: bg `ink`, text `sand-0` `caption`, padding `6px space-2`, `radius-sm`, `shadow-sm`, max-width 240px, 6px arrow. Placement auto (top preferred), 8px offset. Entrance fade+scale .96, `duration-fast`. `role="tooltip"`, trigger `aria-describedby`; shows on hover AND focus; `Esc` dismisses; never contains interactive content (use popover pattern via Drawer/Modal instead).

### 3.18 Tabs

Underline style (default): row border-bottom `border`; tab padding `10px space-4`, `label` `ink-muted`, gap `space-1`. Hover: text `ink`. Active: text `primary`, weight 600, 2px bottom bar `primary` (slides between tabs, 200ms `ease-in-out`). Focus ring inset.
Pill style (view switchers, TimeSlotPicker filters): container `surface-sunken radius-md` padding 4px; active tab `surface-raised radius-sm shadow-xs` text `ink`.
Keyboard: ARIA tabs — ←/→ move (activation follows focus), `Home/End`; `role="tablist/tab/tabpanel"`, `aria-selected`, `aria-controls`. Overflow: horizontal scroll with 16px fade masks, no wrapping.

### 3.19 Accordion (FAQ)

Rows separated by 1px `border` (no boxes). Header button: full-width, padding `space-4 0`, `heading-sm ink` left, `Plus` 20px `ink-muted` right rotating 45°→`X` on open (`duration-base`). Hover: text `primary`. Open: header weight 600. Panel: `body-md ink-muted`, padding-bottom `space-5`, max-width 68ch. Height animation 250ms `ease-in-out` (reduced-motion: crossfade). `aria-expanded`, `aria-controls`, `h3` headers for SEO on marketing FAQ. Single-open by default on marketing; multi-open allowed in portal help.

### 3.20 Badge / Chip

Badge (status): `micro` (11px, uppercase, +0.08em), padding `3px 8px`, `radius-full`, status-bg/status-ink pairs from §1 (e.g. "Confirmed" `success-bg`/`success-ink`). Optional 6px dot instead of icon.
Chip (filter/tag input): height 28px, `radius-full`, border `border-strong`, bg `surface-raised`, `caption ink`, optional avatar 18px; selected: bg `eucalyptus-100` border `primary`; removable chips get `X` 14px target ≥24px. Hover: border `ink-faint`. Chip groups wrap with 8px gaps.

### 3.21 Card

Default: bg `surface-raised`, border 1px `border`, `radius-lg`, padding `space-6`, `shadow-none`. Hoverable/clickable card: hover border `border-strong` + `shadow-sm` + translateY(-2px), `duration-base ease-out`; whole card is one `<a>`/`<button>` (no nested interactive elements). Marketing feature card: `radius-xl`, padding `space-8`, optional top image with `radius-xl` inner crop. Stat card (admin): label `micro ink-faint`, value `display-sm` Figtree 600, delta `caption` with `TrendingUp/Down` in `success`/`danger`. Selected card (RadioCard, §3.8): 1.5px `primary` border + `eucalyptus-50`.

**Terios card families:** cards do not all share one silhouette. Choice cards use the asymmetric care-menu composition: numbered treatment rail, editorial name and description, duration capsule, separated price/action compartment, and a dark eucalyptus selected state with an explicit check. Record cards (sessions, forms, documents, payments, reviews) are quieter, with a rounded upper-right shoulder, care-point marker, and small hover lift. Booking summaries use a labelled header strip and inset reassurance note. Quote cards use alternating radii and a large tonal quotation mark. Error and empty states remain visually quiet; they do not inherit interactive card movement.

### 3.22 DataTable (admin)

Container: `surface-raised radius-lg border`, overflow-x auto with custom scrollbar. Header: `caption` weight 600 `ink-muted`, bg `surface-sunken`, sticky, 1px bottom `border`; sortable columns get `ArrowUpDown` 14px, active sort `primary` + direction arrow, `aria-sort`. Rows: height 52px, `body-sm`, border-bottom `border`; hover bg `eucalyptus-50` (whole row clickable if row action exists, cursor pointer); selected (bulk): bg `eucalyptus-100` + Checkbox in first column (sticky left with name column on scroll). Cells: padding `0 space-4`; status via Badge; money/dates right-aligned with `tnum`; actions column right: IconButton ghost `MoreHorizontal` → dropdown menu (Select popup styling: items 36px, danger item text `danger`).

- Empty: EmptyState (§25) inside table body. Loading: 5 Skeleton rows matching cell shapes. Bulk bar: appears bottom-sticky inside container when ≥1 selected — `surface-raised shadow-md border radius-lg`, "3 selected" + actions.
- `role="table"` semantics via real `<table>`; row action `Enter`; column resize not in v1.

### 3.23 Pagination

Row centered under table/list, gap `space-1`: prev/next IconButtons (`ChevronLeft/Right`, outline sm, disabled at ends) + page buttons 32px `radius-md` `body-sm tnum`: default ghost `ink-muted`, hover `surface-sunken`, current `surface-sunken` + `ink` weight 600 + 1px `border-strong` border (not filled primary — too loud for admin). Ellipsis as static `…`. "Page 2 of 9 · 87 clients" `caption ink-faint` right-aligned. Keyboard: buttons are buttons; `aria-label="Go to page 3"`, current `aria-current="page"`.

### 3.24 SignaturePad (consent forms)

Canvas 100% width × 200px, `surface-raised`, 1.5px dashed `border-strong`, `radius-lg`; placeholder `caption ink-faint` "Sign here" + baseline guide (1px `border` 48px from bottom). Stroke: 2.5px `ink`, round caps, velocity-sensitive width 1.5–3.5px, pointer events (mouse + touch + pen). Once drawn: border solid `border-strong`, placeholder hidden. Footer row: "Clear" ghost sm (resets, `aria-live` announces), helper `caption ink-faint` "Use your mouse or finger". Empty submit → error border `danger` + inline error. Exports PNG (transparent bg, 2× scale) + stroke JSON. Focusable with `role="img" aria-label="Signature pad"`; keyboard alternative: "Type signature" toggle rendering typed name in Figtree italic 28px as legal equivalent.

### 3.25 StarRating

Stars: Lucide `Star`, filled `accent` (`clay-500`), empty `border-strong` outline, 20px default (display 24px, input 28px). Input mode: `role="radiogroup"`, each star `role="radio"` with label "1 star"…; hover previews fill up to hovered star (`duration-instant`), focus ring on star group, ←/→ adjust. Fractional display: clip-path partial fill. With count: `(4.9)` `caption ink-muted` 6px right. Disabled/readonly: no hover, `aria-readonly`.

### 3.26 VideoRoom chrome (session UI)

Full-viewport surface `eucalyptus-900` (dark regardless of theme — video needs neutral dark; this is the one sanctioned dark surface in the customer app). Remote video: full-bleed, `object-fit: cover`.

- **Controls bar**: bottom-centered, floating, bg `rgba(16,21,18,.72)` + `backdrop-filter: blur(12px)`, `radius-full`, padding `space-2`, gap 8px, auto-hides after 4s idle (fade 250ms; always visible when focus within or pointer near). Buttons: 48px `radius-full` IconButtons — mic `Mic/MicOff`, camera `Video/VideoOff`, screen share `ScreenShare`, chat `MessageSquare` (unread dot 8px `accent`), leave `PhoneOff`. Default: bg transparent, icon `sand-0`; hover `rgba(253,252,250,.12)`. Muted/off state: bg `sand-0` icon `ink` (inverted = "you turned this off"). Leave: bg `danger` icon `on-primary`, hover `danger-hover`, requires confirm Modal ("Leave this session?"). All have tooltips + `aria-pressed`.
- **Self-view tile**: 180×120px (mobile 96×128 portrait), top-right 16px inset, `radius-md`, `shadow-lg`, mirror transform, draggable; click swaps with main view. Camera-off: `surface-sunken` + initials avatar (40px, `eucalyptus-700` bg, Figtree 18px `on-primary`).
- **Connection states**: pill top-left — good: `success` dot + "Good connection"; poor: `warning` dot + "Connection unstable — audio only suggested"; reconnecting: full scrim `overlay` + `LoaderCircle` + "Reconnecting… we'll keep trying for 2:00" countdown; failed: EmptyState pattern with "Rejoin" primary Button. Pre-join lobby: camera preview card `radius-xl`, device Selects (custom), "Join session" primary lg.
- Timer: elapsed `caption tnum sand-0/70` next to connection pill. Recording indicator (admin-initiated): 8px `danger` pulsing dot (1200ms) + "REC" `micro`.

### 3.27 EmptyState

Centered column, max-width 360px, padding `space-12 space-6`: Lucide icon 32px `ink-faint` inside 64px `radius-full` `surface-sunken` well, title `display-sm` (portal/marketing) or `heading-md` (admin), body `body-sm ink-muted` (voice: say what will appear, e.g. "When you book a session, it will appear here."), optional primary/ghost action. No illustration, no dashed-border boxes. Table variant: reduced padding, no icon well.

### 3.28 Skeleton loaders

Shapes: text line (height = font-size × line-height, `radius-sm`), circle, block (`radius-md`/`lg` matching target). Base `surface-sunken`; shimmer: linear-gradient 90deg transparent → `sand-0` at 60% → transparent, 200px band sweeping 1600ms linear infinite (dark: `surface-raised` base, `ink` at 6% band). Reduced motion: static, no sweep. Rules: mirror real layout 1:1 (no generic spinner pages); spinner (`LoaderCircle`) only for buttons, inline waits <2s, and VideoRoom reconnect. `aria-busy="true"` on region, `aria-hidden` on skeleton shapes, `role="status"` + visually-hidden "Loading…" text.

### 3.29 Form field wrapper

Anatomy (vertical rhythm): Label `label ink` → 6px → Control → 6px → hint `caption ink-faint` OR error `caption danger-ink` with `CircleAlert` 14px inline. Required marker: `accent` asterisk + visually-hidden "required" (never color-only: also `aria-required`). Optional fields get "(optional)" `caption ink-faint` suffix instead — mark optional, not required, when most fields are required. Error replaces hint (hint moves to `title`-less tooltip on the error icon if still needed). `aria-describedby` links control↔hint/error ids; errors announced via `aria-live`. Horizontal variant (settings rows): label left 200px, control right, divider `border` between rows. Form-level error summary: `danger-bg radius-md` box top of form listing errors as anchor links (`danger-ink` underlined), focus moves to it on submit.

### 3.30 Nav

**Customer top nav** (marketing): at the top of a page it settles as a full-width 76px bar. After 48px of scroll it contracts into a floating 66px frame, max-width 1240px, with 12px viewport inset, `surface` at 92% with blur, `radius-xl`, and a tinted botanical shadow; returning to the top expands it again. The passive scroll listener updates this small boolean directly—scheduling through `requestAnimationFrame` can leave a background/restored tab stuck in its old state. Custom leaf tile and clay care-point form the wordmark. Desktop links sit in a sunken segmented rail; active is a raised capsule with clay point. Right: ghost "Sign in" / primary sm "Book now". Mobile: full-screen eucalyptus chapter menu with numbered editorial rows, fixed actions and a visible close control. Guest booking keeps this public header so the flow never becomes a navigational dead end. Authenticated portal chrome adds an explicit `Back to website` action.

**Marketing footer:** rounded eucalyptus field with an inset ivory booking statement, oversized Figtree close, custom leaf tile, practice-quality markers, and two horizontal navigation rails. Mobile stacks the booking action, wordmark, rails, and legal line without collapsing into a generic link-column grid.

**Legal pages:** Terms and Privacy share an editorial trust-page system rather than a generic prose template. The opening trust panel states the practical summary and review date on eucalyptus; a sticky numbered contents rail anchors into asymmetric reading chapters with restrained icons. The close offers the related policy and a direct contact path. On mobile the contents remains in document flow, chapter metadata becomes a compact horizontal rail, and body copy retains a comfortable `1.8` line height.

**Admin sidebar**: 264px fixed, bg `surface-sunken`, right `border`; wordmark 20px padded `space-5`; section labels `micro ink-faint` padded `space-4 space-5 space-2`; items: height 36px, `radius-md`, margin `0 space-3`, padding `0 space-3`, icon 18px + `label ink-muted`, gap 12px; hover bg `surface-raised`; active bg `eucalyptus-100` text `eucalyptus-800` weight 600 + 3px left bar `primary` (rounded, 20px tall). Bottom: user card (avatar 32px, name `body-sm`, role `caption ink-faint`) + collapse IconButton → 72px rail (icons centered, tooltips right). Mobile: hidden, hamburger opens Drawer variant. Keyboard: full vertical menu pattern, typeahead, `aria-current="page"`.

---

## 4. Cross-cutting states checklist

For any new component not listed: states = default / hover / focus-visible (brand ring) / active / disabled; loading (Spinner or Skeleton, never layout shift); error (danger tokens + text, never color alone); empty (EmptyState pattern); dark theme via tokens only (test both). Touch targets ≥40px. All text from the type scale. All durations from motion tokens. If a rule here conflicts with a library default, the library is restyled or replaced — the contract wins.
