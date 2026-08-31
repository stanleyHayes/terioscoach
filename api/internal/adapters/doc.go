// Package adapters contains inbound adapters (HTTP API, WebSocket
// signaling) and outbound adapters (MongoDB repositories, Resend mailer,
// Stripe payments, Cloudinary storage). Adapters depend on ports and
// domain, never the reverse.
package adapters
