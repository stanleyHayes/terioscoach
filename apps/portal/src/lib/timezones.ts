/**
 * Time zone choices for account and scheduling pickers. The practice runs on
 * US Eastern time, so it is the default and the United States zones are listed
 * first; every other IANA zone stays available below them.
 */

export const DEFAULT_TIME_ZONE = "America/New_York";
export const DEFAULT_TIME_ZONE_LABEL = "Eastern Time (US)";

export interface TimeZoneOption {
  /** IANA name, e.g. "America/New_York". */
  value: string;
  /** Primary line, e.g. "Eastern Time (US)" or "Accra". */
  label: string;
  /** Secondary line, e.g. "New York · GMT-4". */
  detail: string;
  /** Single-line text for the closed picker. */
  display: string;
  /** Lower-cased haystack the search box matches against. */
  search: string;
}

export interface TimeZoneGroup {
  label: string;
  items: TimeZoneOption[];
}

const US_TIME_ZONES: ReadonlyArray<[zone: string, name: string, keywords: string]> = [
  ["America/New_York", "Eastern Time", "est edt et new york miami atlanta boston washington dc"],
  ["America/Chicago", "Central Time", "cst cdt ct chicago houston dallas"],
  ["America/Denver", "Mountain Time", "mst mdt mt denver salt lake city"],
  ["America/Phoenix", "Arizona Time", "mst phoenix arizona"],
  ["America/Los_Angeles", "Pacific Time", "pst pdt pt los angeles san francisco seattle"],
  ["America/Anchorage", "Alaska Time", "akst akdt anchorage"],
  ["Pacific/Honolulu", "Hawaii Time", "hst honolulu"],
];

/** "GMT-4", "GMT+0" — the zone's offset right now. */
export function timeZoneOffset(zone: string, at: Date = new Date()): string {
  try {
    const part = new Intl.DateTimeFormat("en-US", {
      timeZone: zone,
      timeZoneName: "shortOffset",
    })
      .formatToParts(at)
      .find((p) => p.type === "timeZoneName");
    const value = part?.value ?? "GMT";
    return value === "GMT" ? "GMT+0" : value;
  } catch {
    return "";
  }
}

function place(zone: string): { city: string; region: string } {
  const parts = zone.split("/");
  return {
    city: (parts.at(-1) ?? zone).replaceAll("_", " "),
    region: parts.slice(0, -1).join(" / ").replaceAll("_", " "),
  };
}

export function normalizeTimeZoneQuery(query: string): string {
  return query.trim().toLowerCase().replaceAll("_", " ");
}

function allZones(): string[] {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return [];
  }
}

function option(zone: string, name?: string, keywords = ""): TimeZoneOption {
  const offset = timeZoneOffset(zone);
  const { city, region } = place(zone);
  if (zone === "UTC") {
    return {
      value: zone,
      label: "UTC",
      detail: "Coordinated Universal Time · GMT+0",
      display: "UTC · GMT+0",
      search: "utc gmt coordinated universal time gmt+0",
    };
  }
  const label = name ? `${name} (US)` : city;
  const detail = [name ? city : region, offset].filter(Boolean).join(" · ");
  const display = name ? `${label} · ${offset}` : `${city}${region ? `, ${region}` : ""} · ${offset}`;
  return {
    value: zone,
    label,
    detail,
    display,
    search: normalizeTimeZoneQuery(`${label} ${zone} ${city} ${region} ${offset} ${keywords}`),
  };
}

/**
 * United States zones first (Eastern leading), then every other zone
 * alphabetically. `current` is kept even if this browser does not list it.
 */
export function timeZoneGroups(current?: string): TimeZoneGroup[] {
  const us = US_TIME_ZONES.map(([zone, name, keywords]) => option(zone, name, keywords));
  const usZones = new Set(us.map((o) => o.value));
  const rest = Array.from(new Set(["UTC", ...allZones(), ...(current ? [current] : [])]))
    .filter((zone) => !usZones.has(zone))
    .sort((a, b) => (a === "UTC" ? -1 : b === "UTC" ? 1 : a.localeCompare(b)))
    .map((zone) => option(zone));
  return [
    { label: "United States", items: us },
    { label: "All other time zones", items: rest },
  ];
}

export function matchesTimeZone(item: TimeZoneOption, query: string): boolean {
  const q = normalizeTimeZoneQuery(query);
  return q === "" || item.search.includes(q);
}
