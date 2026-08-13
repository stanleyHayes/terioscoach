import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/**/*.test.{ts,tsx}",
        // Next.js entry points that exist to be found by the framework:
        // a layout that renders {children}, a route that re-exports
        // metadata. Counting them measures the framework, not this app.
        "src/app/**/layout.tsx",
        "src/app/**/{sitemap,robots,not-found}.ts?(x)",
      ],
      // A ratchet, set just under what the suite actually achieves
      // today. It fails the build when a change lands with no test at
      // all, which is the case worth catching.
      //
      // These numbers go up and never down. Lowering one to make a build
      // pass converts the gate into a formality, which is worse than
      // having no gate — it reports green while covering less.
      thresholds: {
        lines: 73,
        functions: 72,
        branches: 68,
        statements: 71,
      },
    },
  },
});
