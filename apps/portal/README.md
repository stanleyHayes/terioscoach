# Terios client portal

The private client experience for Terios Wellness Spa. It is intentionally a
separate Next.js workspace so it can deploy at `app.terioscoach.com` without
coupling releases to the public website.

## Local development

From the repository root:

```sh
npm install
npm run dev:portal
```

The default local URL is `http://localhost:3000/portal`. Set these variables in
an untracked `.env.local` when the matching defaults are not suitable:

```dotenv
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_PORTAL_URL=http://localhost:3000
NEXT_PUBLIC_WEBSITE_URL=http://localhost:3001
```

## Verification

```sh
npm run lint --workspace=apps/portal
npm run test:coverage --workspace=apps/portal
npm run build --workspace=apps/portal
```

## Deployment

Create an independent Vercel project with `apps/portal` as its root and attach
`app.terioscoach.com`. The complete environment, CORS, DNS and cutover contract
is documented in `design/go-live-runbook.md`.

Every route is private or authentication-related. `vercel.json` therefore
applies `noindex`, `noarchive` and `no-store` headers throughout the app.
