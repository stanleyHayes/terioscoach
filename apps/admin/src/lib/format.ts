/**
 * Display + conversion helpers for money and durations.
 *
 * Money is integer minor units (kobo) per the API contract; the UI speaks in
 * major units (cedis). Formatting goes through Intl.NumberFormat — for GHS the
 * en-GH locale renders the GH₵ narrow symbol.
 */

export const DEFAULT_CURRENCY = "GHS";

const moneyFormatters = new Map<string, Intl.NumberFormat>();

function moneyFormatter(currency: string): Intl.NumberFormat {
  let formatter = moneyFormatters.get(currency);
  if (!formatter) {
    formatter = new Intl.NumberFormat("en-GH", {
      style: "currency",
      currency,
      currencyDisplay: "narrowSymbol",
    });
    moneyFormatters.set(currency, formatter);
  }
  return formatter;
}

/** 123450, "GHS" → "GH₵1,234.50". */
export function formatMoney(minorUnits: number, currency = DEFAULT_CURRENCY): string {
  return moneyFormatter(currency).format(minorUnits / 100);
}

/**
 * Edit-friendly major-units string: 25000 → "250", 25050 → "250.5",
 * 25005 → "250.05". Used to pre-fill the price field on edit.
 */
export function minorToMajorString(minorUnits: number): string {
  return (minorUnits / 100)
    .toFixed(2)
    .replace(/\.00$/, "")
    .replace(/(\.\d)0$/, "$1");
}

const PRICE_PATTERN = /^\d+(\.\d{1,2})?$/;

/**
 * Parse a major-units price ("250", "250.5", "250.50", "1,250.00") into integer
 * minor units. Returns null for anything that is not a non-negative decimal
 * with at most two fraction digits.
 */
export function parseMajorToMinor(input: string): number | null {
  const normalized = input.trim().replace(/,/g, "");
  if (!PRICE_PATTERN.test(normalized)) {
    return null;
  }
  return Math.round(Number.parseFloat(normalized) * 100);
}

/** 45 → "45 min", 60 → "1 hr", 90 → "1 hr 30 min". */
export function formatDuration(minutes: number): string {
  if (minutes < 60) {
    return `${minutes} min`;
  }
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  const label = `${hours} hr`;
  return rest === 0 ? label : `${label} ${rest} min`;
}
