# Brand asset inventory

## Runtime assets

| Asset | Public path | Intended use |
|---|---|---|
| Theresa clinical portrait | `/images/brand/theresa-yirerong-clinical.webp` | Homepage CMS fallback |
| Theresa founder portrait | `/images/brand/theresa-yirerong-about.webp` | About-page CMS fallback |
| Theresa outdoor portrait | `/images/brand/portraits/theresa-yirerong-by-jinnifer-douglass-062.webp` | Work-with-me CMS fallback |
| Generated wellness scene | `/images/marketing/home-hero.webp` | Optional campaign or CMS replacement |
| Generated practitioner portrait | `/images/marketing/about-practitioner.webp` | Optional campaign or CMS replacement |
| Clean supplied wordmark | `/images/brand/terios-logo.svg` | Archived web-ready logo variant |
| Clean supplied symbol | `/images/brand/terios-mark.svg` | Archived web-ready mark variant |
| Cropped supplied symbol | `/images/brand/identity/terios-mark.svg` | Live public, portal, and dashboard navigation identity |
| Complete supplied portrait library (28 optimized files) | `/images/brand/portraits/*.webp` | CMS-selectable editorial portraits |
| Final supplied logo set (six vectors) | `/images/brand/logo-variants/terios-logo-{1..6}.svg` | Master light/dark/lockup variants for controlled reuse |

The live interface follows `design/brand.md`, whose eucalyptus, clay, and sand system supersedes the older purple/pink palette written in the supplied creative brief. All six vectors from the archive's `Logos/final version` directory are retained. Watermarked Fiverr previews, revision proofs, duplicate raster exports, AI/EPS/PDF production copies, and obsolete logo iterations were intentionally not copied into the runtime bundle because they add weight without adding a distinct web asset.

## CMS image slots

The published CMS pages with slugs `home`, `about`, and `work-with-me` each expose a cover image in Admin > Content > Pages. Uploading a replacement there changes the corresponding marketing image without a deployment. If a page is absent, unpublished, or has no image, the web app uses the local fallback above.

CMS uploads accept JPEG, PNG, and WebP. Prefer WebP, at least 1400 px on the long edge, under 2 MB. Use portrait crops for `home` and `about`; use a landscape crop for `work-with-me`.

The Admin media library exposes the three generated marketing images, all 28 supplied portraits, all 20 supplied blog images, and every image previously uploaded through the CMS. The library is searchable and filterable, and persisted uploads can be reused across pages and posts on any signed-in device. The portrait originals remain out of the runtime bundle; each was resized to a maximum width of 1600 pixels and converted to WebP at quality 82. This reduces the source set from roughly 88 MB to roughly 6.4 MB while keeping enough resolution for CMS hero and article use. The journal listing renders a post's selected cover image, so published editorial images now appear in both the listing and article surfaces.

The supplied cropped symbol is also the single source for the web, client-portal, and practice-dashboard favicons and dashboard identity badges. This avoids the former generic leaf and letter placeholders and keeps browser tabs, authentication screens, and navigation visually consistent.

## Blog image library

Twenty supplied blog images were converted to WebP and stored in `apps/web/public/images/blog`. Their filenames retain the source identifiers for provenance. Suggested routing:

- Food and nutrition: `asparagus`, `garlic`, `grapes`, `meal`, `milkshake`, `tray`, `wine`, `drink`.
- Mindfulness and restoration: `lake`, `lavender`, `swan`, `waterfall`.
- Coaching and lifestyle: the three `woman-*` files, `people`, `girl`, `businessman`, `marketing-online`, `ai-generated`.

These are an editorial library, not automatically published posts. Editors should confirm subject fit, attribution/licensing, alt text, and clinical context before uploading or linking one in the CMS.

The marketing website also uses this retained library as a curated visual layer. Homepage previews, FAQ context, and live service rows draw from approved portraits and editorial images. Service imagery is selected by care theme (introductory conversation, nursing, coaching, mindfulness/recovery, nutrition, or bodywork) with a stable fallback rotation for custom service names, so every published service has an accompanying image without weakening the live catalog contract.

## Legal templates

The supplied nurse-coaching and holistic-coaching agreements are retained in `design/legal/templates`. They are examples with blanks and explicit legal disclaimers. They must be reviewed and customized by qualified counsel before use. They are not public downloads and are not wired into client consent flows.

## Archive disposition

The uploaded archive was fully expanded and inventoried on 30 August 2026. Its usable content is represented as follows:

- 28 headshots: all retained as optimized CMS-selectable WebP files.
- 20 blog photographs: all retained as optimized CMS-selectable WebP files.
- Six final SVG logo variants: all retained; the primary wordmark and mark are also available at their stable public paths.
- Nurse and holistic coaching agreements: retained under `design/legal/templates` for legal review.
- `My Creative Assets.docx`: used as a source brief; its runtime-relevant guidance is represented in `design/brand.md` and this inventory.
- `Practice Better Setup Flowsheet.docx`: treated as a source workflow reference; the implemented booking, forms, payments, documents, and client-record flows supersede it and are tracked in `agent_plan.md`.
- Fiverr preview images, revisions, stationery mockups, nested social ZIPs, and duplicate PNG/JPG/PDF/AI/EPS exports: intentionally excluded from production runtime because they are previews, duplicates, watermarked material, or source-format masters.

After this inventory and the corresponding runtime files are verified, the source ZIP is removed from the repository root so it cannot inflate Git history or deployments.
