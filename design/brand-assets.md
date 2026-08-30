# Brand asset inventory

## Runtime assets

| Asset | Public path | Intended use |
|---|---|---|
| Theresa clinical portrait | `/images/brand/theresa-yirerong-clinical.webp` | Homepage CMS fallback |
| Theresa founder portrait | `/images/brand/theresa-yirerong-about.webp` | About-page CMS fallback |
| Generated consultation still life | `/images/marketing/services-care.webp` | Work-with-me CMS fallback |
| Generated wellness scene | `/images/marketing/home-hero.webp` | Optional campaign or CMS replacement |
| Generated practitioner portrait | `/images/marketing/about-practitioner.webp` | Optional campaign or CMS replacement |
| Clean supplied wordmark | `/images/brand/terios-logo.svg` | Archived web-ready logo variant |
| Clean supplied symbol | `/images/brand/terios-mark.svg` | Archived web-ready mark variant |

The live interface follows `design/brand.md`, whose eucalyptus, clay, and sand system supersedes the older purple/pink palette written in the supplied creative brief. The clean supplied SVGs are retained for reference and future controlled use; watermarked Fiverr previews, revisions, duplicate exports, AI/EPS production files, and obsolete logo iterations were intentionally not copied into the runtime bundle.

## CMS image slots

The published CMS pages with slugs `home`, `about`, and `work-with-me` each expose a cover image in Admin > Content > Pages. Uploading a replacement there changes the corresponding marketing image without a deployment. If a page is absent, unpublished, or has no image, the web app uses the local fallback above.

CMS uploads accept JPEG, PNG, and WebP. Prefer WebP, at least 1400 px on the long edge, under 2 MB. Use portrait crops for `home` and `about`; use a landscape crop for `work-with-me`.

## Blog image library

Twenty supplied blog images were converted to WebP and stored in `apps/web/public/images/blog`. Their filenames retain the source identifiers for provenance. Suggested routing:

- Food and nutrition: `asparagus`, `garlic`, `grapes`, `meal`, `milkshake`, `tray`, `wine`, `drink`.
- Mindfulness and restoration: `lake`, `lavender`, `swan`, `waterfall`.
- Coaching and lifestyle: the three `woman-*` files, `people`, `girl`, `businessman`, `marketing-online`, `ai-generated`.

These are an editorial library, not automatically published posts. Editors should confirm subject fit, attribution/licensing, alt text, and clinical context before uploading or linking one in the CMS.

## Legal templates

The supplied nurse-coaching and holistic-coaching agreements are retained in `design/legal/templates`. They are examples with blanks and explicit legal disclaimers. They must be reviewed and customized by qualified counsel before use. They are not public downloads and are not wired into client consent flows.
