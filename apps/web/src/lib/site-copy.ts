import { getPage } from "./content";
import {
  contentSlug,
  legacySiteValues,
  parseSiteValues,
  siteCopy,
} from "../../../../shared/site-content";
export async function getSiteValues(scope: string) {
  const [page, legacy] = await Promise.all([
    getPage(contentSlug(scope)).catch(() => undefined),
    ["home", "about", "work-with-me"].includes(scope)
      ? getPage(scope).catch(() => undefined)
      : undefined,
  ]);
  return {
    ...legacySiteValues(scope, legacy),
    ...parseSiteValues(page?.body ?? ""),
  };
}
export async function getSiteCopy(scope: string) {
  return siteCopy(scope, await getSiteValues(scope));
}
