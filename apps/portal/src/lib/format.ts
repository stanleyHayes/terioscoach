/**
 * Display formatters shared by the marketing pages (WEB-04).
 * Money arrives from the API as integer minor units + ISO 4217 code
 * (contract Conventions); durations arrive as minutes.
 */

/** Minor units + ISO 4217 → localized display, e.g. (45000, "USD") → "$450.00". */
export function formatMoney(minorUnits: number, currency = "USD"): string {
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency,
      currencyDisplay: "narrowSymbol",
    }).format(minorUnits / 100);
  } catch {
    // Unknown/invalid ISO code — degrade to code + decimals instead of throwing.
    return `${currency} ${(minorUnits / 100).toFixed(2)}`;
  }
}

/** Minutes → human duration: 45 → "45 min", 60 → "1 h", 90 → "1 h 30 min". */
export function formatDuration(minutes: number): string {
  if (minutes < 60) {
    return `${minutes} min`;
  }
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder === 0 ? `${hours} h` : `${hours} h ${remainder} min`;
}

/* ---------------------------------------------------------------------------
 * Scheduling formatters. Every scheduling surface states its timezone
 * explicitly (design-system contract) — pair these with gmtOffsetLabel.
 * `timeZone` is an IANA name; inputs are RFC 3339 UTC instants.
 * ------------------------------------------------------------------------- */

/** UTC instant → wall-clock time in `timeZone`: "9:30 AM", "3:30 PM". */
export function formatTimeOfDay(isoUtc: string, timeZone: string): string {
  return new Intl.DateTimeFormat("en-US", {
    hour: "numeric",
    minute: "2-digit",
    timeZone,
  }).format(new Date(isoUtc));
}

/** UTC instant → wall-clock range in `timeZone`: "9:30 AM – 10:15 AM". */
export function formatTimeRange(startIsoUtc: string, endIsoUtc: string, timeZone: string): string {
  return `${formatTimeOfDay(startIsoUtc, timeZone)} – ${formatTimeOfDay(endIsoUtc, timeZone)}`;
}

/** UTC instant → calendar date in `timeZone`: "Tue, Aug 12, 2026". */
export function formatSessionDate(isoUtc: string, timeZone: string): string {
  return new Intl.DateTimeFormat("en-US", {
    weekday: "short",
    month: "short",
    day: "numeric",
    year: "numeric",
    timeZone,
  }).format(new Date(isoUtc));
}

/** IANA zone → explicit offset label for scheduling surfaces: "GMT+0",
 * "GMT-4", "GMT+5:30". `at` pins the instant the offset is computed for
 * (defaults to now) so DST zones show the offset that actually applies. */
export function gmtOffsetLabel(timeZone: string, at: Date = new Date()): string {
  try {
    const part = new Intl.DateTimeFormat("en-US", {
      timeZone,
      timeZoneName: "shortOffset",
    })
      .formatToParts(at)
      .find((p) => p.type === "timeZoneName");
    const value = part?.value ?? "GMT";
    // Engines render zero offset as bare "GMT"; scheduling surfaces state the
    // offset explicitly, so normalize to "GMT+0".
    return value === "GMT" ? "GMT+0" : value;
  } catch {
    return timeZone;
  }
}

/** The visitor's own IANA timezone (browser-reported). */
export function browserTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}
