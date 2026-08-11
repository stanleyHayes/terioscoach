/** Join class names, dropping falsy values. The design system bans raw hex,
 * not utility classes — this is the standard class composition helper. */
export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}
