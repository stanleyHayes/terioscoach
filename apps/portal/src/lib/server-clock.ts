let anchor: { server: number; monotonic: number } | null = null;
export function synchronizeClock(iso: string) {
  const server = Date.parse(iso);
  if (Number.isFinite(server))
    anchor = { server, monotonic: performance.now() };
}
/** Before the first response use local time; scheduling lists are still loading then. */
export function serverNow() {
  return new Date(
    anchor ? anchor.server + performance.now() - anchor.monotonic : Date.now(),
  );
}
