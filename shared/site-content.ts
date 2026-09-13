import definitions from "./website-content.json";
export type ContentField = {
  key: string;
  label: string;
  group: string;
  type: string;
  value: string;
};
export const websiteContent: Record<
  string,
  { label: string; fields: ContentField[] }
> = definitions;
export const contentSlug = (scope: string) => `website-content-${scope}`;
export type SiteValues = Record<string, string>;
export function parseSiteValues(body: string): SiteValues {
  try {
    const value: unknown = JSON.parse(body);
    if (!value || typeof value !== "object" || Array.isArray(value)) return {};
    return Object.fromEntries(
      Object.entries(value).filter(
        (entry): entry is [string, string] => typeof entry[1] === "string",
      ),
    );
  } catch {
    return {};
  }
}
export function validContentUrl(value: string, image = false): boolean {
  if (!value) return true;
  if (value.startsWith("/") && !value.startsWith("//") && !value.includes("\\"))
    return true;
  try {
    const url = new URL(value);
    return image
      ? url.protocol === "https:"
      : ["https:", "mailto:", "tel:"].includes(url.protocol);
  } catch {
    return value.startsWith("#") && !image;
  }
}
export function siteCopy(scope: string, values: SiteValues = {}) {
  return (key: string, fallback: string): string => {
    const field = websiteContent[scope]?.fields.find(
      (field) => field.key === key,
    );
    const value = values[key];
    if (typeof value !== "string") return fallback;
    if (field?.type === "image" && !validContentUrl(value, true))
      return fallback;
    if (field?.type === "link" && !validContentUrl(value)) return fallback;
    return value;
  };
}

/** Preserve published content from the earlier, limited page editor. */
export function legacySiteValues(
  scope: string,
  page?: { body?: string; coverImage?: string },
): SiteValues {
  const values: SiteValues = {};
  if (!page) return values;
  const coverKeys: Record<string, string> = {
    home: "field-017",
    about: "field-027",
    "work-with-me": "field-010",
  };
  if (page.coverImage && coverKeys[scope])
    values[coverKeys[scope]] = page.coverImage;
  if (page.body) {
    if (scope === "home")
      values["hero-description"] =
        page.body.split(/\n\s*\n/).find(Boolean) || "";
    if (scope === "work-with-me") values["field-009"] = page.body;
    if (scope === "about") values["practitioner-story"] = page.body;
  }
  return values;
}
