// Custom-page existence must resolve before any loading shell sends HTTP 200.
// Reuse the public site chrome without inheriting marketing/loading.tsx.
export { default } from "../(marketing)/layout";
