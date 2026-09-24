# PDF letterhead artwork

`terios-logo.png` is the original full logo from
`apps/web/public/images/brand/terios-logo.svg`, rendered with `rsvg-convert`
at 1200px canvas width, cropped to the nontransparent bounds, and composited
onto white. Colors, lettering and proportions are preserved. This bundled
asset keeps PDF generation independent of network requests or frontend files.

The PDF embeds the image once and reuses it on every page. To update the logo,
regenerate this asset from the approved SVG rather than recreating its shapes.
