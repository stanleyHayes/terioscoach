import { vi } from "vitest";

// Node 26 ships an experimental global `localStorage`/`sessionStorage` that
// vitest's jsdom environment refuses to override, leaving the broken Node
// versions (undefined without --localstorage-file) in place — and since
// `window === global` under vitest, jsdom's real storage is unreachable.
// Stub both globals with a spec-shaped in-memory Storage for tests.
class MemoryStorage implements Storage {
  private store = new Map<string, string>();

  get length(): number {
    return this.store.size;
  }

  clear(): void {
    this.store.clear();
  }

  getItem(key: string): string | null {
    return this.store.has(key) ? this.store.get(key)! : null;
  }

  key(index: number): string | null {
    return [...this.store.keys()][index] ?? null;
  }

  removeItem(key: string): void {
    this.store.delete(key);
  }

  setItem(key: string, value: string): void {
    this.store.set(key, String(value));
  }
}

vi.stubGlobal("localStorage", new MemoryStorage());
vi.stubGlobal("sessionStorage", new MemoryStorage());
