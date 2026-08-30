/**
 * Validates a `?next=` post-auth redirect target: same-site absolute paths
 * only (rejects external URLs and protocol-relative "//host" values).
 * Returns null for anything unsafe, so callers fall back to /portal.
 */
export function safeNextPath(value: string | null): string | null {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return null;
  }
  return value;
}
