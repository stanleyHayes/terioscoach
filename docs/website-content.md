# Website content

Admin **Content → Website sections** controls text, links and images for all nine main marketing pages, navigation, footer, page introductions, contact form, FAQ search and shared legal copy. Related service cards, posts, FAQs, testimonials and reviews keep their existing dedicated editors.

The existing layouts remain in the web components. Their stable field keys and editable defaults live in `shared/website-content.json`. `shared/site-content.ts` validates link/image schemes and imports legacy published Home/About/Work-with-me copy. Page values are stored using the authenticated CMS page API under `website-content-{scope}`. These records are hidden from the ordinary page list and cannot render as custom public pages. Saves publish explicitly; subsequent published edits go live immediately through no-store reads. The editor preserves unsaved work when changing pages and retains record IDs if a publish request fails.

**Content → Pages** creates ordinary Markdown pages at `/{slug}`. A page must be published; missing and draft pages return 404. Cover images and metadata use the saved page fields. Existing built-in URLs keep their designed layouts and use Website sections for section editing.

## Practice service catalog

The public service catalog spans the single Terios practice. Admin list/update/delete routes use the same practice-wide scope, gated by practitioner role or staff `services.manage` permission. Creation retains the creating practitioner ID so bookings and availability keep their correct owner. The application service continues enforcing ownership for callers that supply a practitioner scope.

Production provisioning no longer inserts service offerings. On API startup, `RetireLaunchSample` runs a one-time migration that retires only the exact unchanged launch introduction (name, description, duration, price and currency must match). Modified offerings are preserved. The service ID and booking history remain, and the separate `schema_migrations` marker lets admin explicitly reactivate it after migration. Retired or deleted services are absent from the public API and therefore all three marketing service lists.
