/**
 * HeroWatermark — the botanical field behind the hero.
 *
 * The hero was a flat wash of eucalyptus-900 with only the grain overlay on
 * it, which reads as solid rather than composed. These are the brand's own
 * forms — the mark's frond, its seed circles — blown up well past legibility
 * and held at a few percent opacity, so they register as texture and depth
 * rather than as logos scattered across the page.
 *
 * Purely decorative: aria-hidden, pointer-events-none, and behind the
 * content's stacking order. Placement is in percentages so the composition
 * holds from a phone to an ultrawide; the hero's own `overflow-hidden` crops
 * whatever runs past the edge, which is the intended effect.
 *
 * The drift animation is switched off globally under
 * `prefers-reduced-motion: reduce` (globals.css §reduced motion).
 */

/** One frond of the mark, redrawn as a single closed path. */
function Frond({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 100 140" fill="none" aria-hidden="true" className={className}>
      <path
        d="M50 138C50 138 6 96 6 54C6 26 26 4 50 2C74 4 94 26 94 54C94 96 50 138 50 138Z"
        fill="currentColor"
      />
      <path
        d="M50 6V134"
        stroke="var(--color-eucalyptus-900, #16281F)"
        strokeOpacity=".35"
        strokeWidth="3"
      />
    </svg>
  );
}

/** A seed ring — the mark's punctuation, used here as open counterform. */
function Ring({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 100 100" fill="none" aria-hidden="true" className={className}>
      <circle cx="50" cy="50" r="46" stroke="currentColor" strokeWidth="2.5" />
    </svg>
  );
}

export function HeroWatermark() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute inset-0 overflow-hidden select-none"
    >
      {/* Top-left, rotated away from the headline so the type stays clean. */}
      <Frond className="botanical-drift absolute -top-[18%] -left-[22%] h-[68%] w-auto -rotate-[24deg] text-eucalyptus-700/45 sm:-left-[16%]" />

      {/* Bottom-right anchor, the largest form, mostly cropped. */}
      <Frond className="absolute -right-[14%] -bottom-[26%] h-[78%] w-auto rotate-[160deg] text-eucalyptus-700/55" />

      {/* A small one riding the lower edge, for rhythm rather than mass. */}
      <Frond className="absolute bottom-[4%] left-[36%] hidden h-[26%] w-auto rotate-[12deg] text-eucalyptus-700/35 lg:block" />

      <Ring className="absolute top-[14%] right-[8%] size-[26vw] max-w-[340px] text-eucalyptus-600/45" />
      <Ring className="absolute top-[6%] right-[26%] hidden size-[12vw] max-w-[150px] text-clay-300/15 lg:block" />
      <Ring className="absolute bottom-[18%] left-[8%] size-[16vw] max-w-[200px] text-eucalyptus-600/35" />

      {/* A soft radial lift under the headline, so the top-left corner is not
          uniformly dark next to the brighter right-hand side. */}
      <div className="absolute -top-[20%] -left-[10%] size-[70%] rounded-full bg-eucalyptus-700/30 blur-[120px]" />
    </div>
  );
}
