# Next.js Frontend

Next.js 16 frontend with [Better Auth](https://better-auth.com) authentication, wired to a Go API backend via a server-side proxy.

## Setup

### 1. Environment

```bash
cp .env.example .env
```

Edit `.env`:
- `BETTER_AUTH_SECRET` — generate with `openssl rand -base64 32`
- `DATABASE_URL` — your Neon Postgres connection string
- `GO_API_URL` / `NEXT_PUBLIC_GO_API_URL` — Go API URL (defaults to `http://localhost:8080`)

### 2. Install & run

```bash
npm install
npx prisma generate
npx prisma db push    # creates User, Session, Account, Verification, Jwks tables
npm run dev
```

The app runs on `http://localhost:3000`.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BETTER_AUTH_SECRET` | Encryption secret (min 32 chars) | — |
| `BETTER_AUTH_URL` | App base URL (server-side) | — |
| `NEXT_PUBLIC_API_URL` | App base URL (client-side) | — |
| `DATABASE_URL` | Neon Postgres connection string | — |
| `GO_API_URL` | Go API URL (server-side proxy) | `http://localhost:8080` |
| `NEXT_PUBLIC_GO_API_URL` | Go API URL (client-side) | `http://localhost:8080` |

## Key Files

| File | Purpose |
|------|---------|
| `lib/auth.ts` | Better Auth server config (Prisma adapter, JWT + email/password plugins) |
| `lib/auth-client.ts` | Better Auth React client (jwtClient plugin, exports `signIn`/`signUp`/`signOut`) |
| `lib/prisma.ts` | Prisma client singleton (v7 driver adapter) |
| `prisma/schema.prisma` | User, Session, Account, Verification, Jwks models |
| `app/api/auth/[...all]/route.ts` | Better Auth route handler |
| `app/api/[...path]/route.ts` | Proxy to Go API (injects JWT server-side) |
| `app/(auth)/register/page.tsx` | Sign-up page |
| `app/(auth)/login/page.tsx` | Sign-in page |
| `app/profile/page.tsx` | Protected profile page (session + Go API test) |

## Routes

| Path | Description |
|------|-------------|
| `/` | Home |
| `/register` | Sign-up form |
| `/login` | Sign-in form |
| `/profile` | Protected — shows session JSON + Go API test buttons |
| `/api/auth/*` | Better Auth endpoints |
| `/api/*` | Proxied to Go API with JWT injection |
