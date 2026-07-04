# Better Auth Go

A full-stack authentication demo pairing a **Next.js + Better Auth** frontend with a **Go** API backend — proving you don't have to abandon Go just because [Better Auth](https://better-auth.com) is TypeScript-only.

## Why this exists

Go is a pragmatic language for building APIs, but its auth story is thin compared to web-first frameworks like Rails or Laravel. Packages like [Authboss](https://github.com/volatiletech/authboss) exist but leave most of the integration (database, endpoints, sessions) to you. Hosted providers like Clerk or Auth0 solve it — but features like 2FA can cost $100/month.

[Better Auth](https://better-auth.com) is a free, open-source TypeScript auth library with first-class support for email/password, OAuth, 2FA, organizations, magic links, and more — all via a plugin system. The catch: it only runs on TypeScript.

This project shows how to keep using Go for your API while using Better Auth for auth, by bridging the two with **JWTs** and a **server-side proxy**.

## How it works

Better Auth's [JWT plugin](https://better-auth.com/docs/plugins/jwt) mints signed JSON Web Tokens (Ed25519) for authenticated users. The Go API verifies those tokens against the public key published at Better Auth's `/api/auth/jwks` endpoint — no shared database or secret required.

```
Browser
  │
  ▼
Next.js (port 3000)
  ├── Better Auth (Prisma + Neon Postgres)
  │     ├── /api/auth/*    — sign-up, sign-in, sign-out, session, jwks, token
  │     └── /api/[...path] — proxy to Go API (injects JWT server-side)
  │
  ▼
Go API (port 8080)
  ├── /api/verify  — verifies JWT from Authorization header
  ├── /api/me      — same handler, returns user info
  └── /health      — health check
```

**Auth flow:**
1. User signs up / signs in via Better Auth endpoints on Next.js
2. Browser receives a session cookie (the JWT never touches the browser)
3. Browser calls `/api/verify` (or `/api/me`) on Next.js
4. The Next.js proxy reads the session cookie and mints a JWT via `auth.api.getToken()`
5. Proxy forwards the request to Go with `Authorization: Bearer <jwt>`
6. Go fetches the public key from Next.js `/api/auth/jwks` and verifies the JWT signature
7. Go returns the user data; proxy forwards the response back to the browser

### Why proxy instead of client-direct?

The transcript walks through three approaches, in order of preference:

1. **Client-direct (simplest, not recommended)** — browser fetches a JWT from `authClient.token()` and sends it straight to Go. Downsides: requires CORS on the Go API, exposes the JWT to the browser (XSS risk if stored in localStorage), and doubles request count (token + API call) unless you cache.
2. **Client-direct with caching** — same as above, but cache the JWT in memory until expiry (see `lib/go-api-client.ts`). Solves the double-request problem; still needs CORS and still exposes the JWT to the browser.
3. **Server-side proxy (recommended, used here)** — browser calls Next.js, which mints the JWT server-side and forwards it to Go. The JWT never reaches the browser, no CORS needed on Go, no client-side token caching. The one caveat is a double hop (browser → Next.js → Go), which is only a real issue if the servers aren't collocated (e.g., Vercel + separate Go host).

This project ships **both** approaches — the proxy route at `app/api/[...path]/route.ts` (used by the profile page's test component) and the client-side `lib/go-api-client.ts` with token caching (kept for reference).

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | Next.js 16, React 19, Tailwind CSS 4, shadcn/ui |
| Auth | Better Auth 1.6 (email/password + JWT plugin) |
| Database | Neon Postgres (serverless PostgreSQL) |
| ORM | Prisma 7 (driver adapter: `@prisma/adapter-pg`) |
| Backend | Go 1.26, [`lestrrat-go/jwx`](https://github.com/lestrrat-go/jwx) for JWT verification |
| Proxy | Next.js route handler (`app/api/[...path]/route.ts`) |

## Project Structure

```
better-auth-go/
├── api/                       # Go API backend
│   ├── auth/auth.go           # JWT verification via JWKS fetch
│   ├── main.go                # HTTP server + routes + CORS
│   ├── middleware/            # Logging middleware
│   ├── .env.example           # Go env vars template
│   └── go.mod
│
├── app/                       # Next.js frontend
│   ├── app/
│   │   ├── (auth)/            # Auth pages (login, register)
│   │   ├── profile/           # Protected profile page + Go API test
│   │   ├── api/auth/[...all]/ # Better Auth route handler
│   │   └── api/[...path]/     # Proxy to Go API (server-side JWT injection)
│   ├── components/
│   │   ├── register-form.tsx  # Sign-up form
│   │   ├── login-form.tsx     # Sign-in form
│   │   ├── sign-out-button.tsx
│   │   └── go-api-test.tsx    # Tests the proxy → Go chain from the browser
│   ├── lib/
│   │   ├── auth.ts            # Better Auth server config (jwt + emailAndPassword plugins)
│   │   ├── auth-client.ts     # Better Auth React client (jwtClient plugin)
│   │   ├── prisma.ts          # Prisma client singleton (v7 driver adapter)
│   │   └── go-api-client.ts   # Optional client-side Go API client with token caching
│   ├── prisma/
│   │   └── schema.prisma      # User, Session, Account, Verification, Jwks
│   ├── .env.example           # Next.js env vars template
│   └── package.json
│
└── README.md
```

## Getting Started

### Prerequisites

- Node.js 20.19+
- Go 1.26+
- A [Neon](https://neon.tech) Postgres database (free tier works)

Check the individual README files in each project directory for specific setup instructions:

- **[Frontend Setup](app/README.md)** — Next.js + Better Auth + Prisma
- **[Backend Setup](api/README.md)** — Go API + JWT verification

### Quick start

```bash
# 1. Start the frontend (terminal 1)
cd app && cp .env.example .env   # edit .env first!
npm install && npx prisma generate && npx prisma db push && npm run dev

# 2. Start the Go API (terminal 2)
cd api && cp .env.example .env
go mod download && go run .
```

### Verify

```bash
curl http://localhost:3000/api/auth/ok        # {"ok":true}
curl http://localhost:8080/health             # {"status":"healthy",...}
curl http://localhost:3000/api/auth/jwks      # {"keys":[{"alg":"EdDSA",...}]}
```

Then open `http://localhost:3000/register`, create an account, and visit `http://localhost:3000/profile` — you'll see your session and a button to test the Go API end-to-end through the proxy.

## Environment Variables

### Next.js (`app/.env`)

| Variable | Description | Default |
|----------|-------------|---------|
| `BETTER_AUTH_SECRET` | Encryption secret (min 32 chars). Generate: `openssl rand -base64 32` | — |
| `BETTER_AUTH_URL` | App base URL (read by Better Auth server-side) | — |
| `NEXT_PUBLIC_API_URL` | App base URL (read by auth client in the browser) | — |
| `DATABASE_URL` | Neon Postgres connection string | — |
| `GO_API_URL` | Go API URL (server-side, used by the proxy route) | `http://localhost:8080` |
| `NEXT_PUBLIC_GO_API_URL` | Go API URL (client-side, used by `go-api-client.ts`) | `http://localhost:8080` |

### Go API (`api/.env`)

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `JWKS_URL` | Better Auth JWKS endpoint (public key discovery) | `http://localhost:3000/api/auth/jwks` |

## Production Notes

- **JWKS caching:** the Go API currently fetches the JWKS on every request. For production, use [`jwk.NewCache`](https://github.com/lestrrat-go/jwx) to cache the key set and refresh periodically.
- **CORS:** the proxy approach means the Go API doesn't need CORS for browser requests. The `cors.AllowAll` handler in `main.go` is only needed if you use the client-direct approach.
- **Key rotation:** Better Auth supports JWKS rotation via `jwks.rotationInterval`. During rotation, both old and new keys are published so existing JWTs keep verifying through a grace period (default 30 days).
- **Collocation:** the proxy adds a hop (browser → Next.js → Go). If both servers run on the same VPS this is negligible; if Next.js is on Vercel and Go is elsewhere, consider the latency.

## Shared vs Separate Database

The Go API currently has no data layer — it only verifies JWTs. But once you add Go-side tables (orders, posts, etc.), you have a decision to make about where the auth tables live relative to your app tables.

### Shared database (this project's default)

All tables — Better Auth's (`user`, `session`, `account`, `verification`, `jwks`) and your Go API's (`orders`, `products`, etc.) — live in the same Neon database. Prisma manages the auth tables; Go manages its own tables via `database/sql` or an ORM like `sqlx`/`pgx`.

```
Neon Postgres (one database)
├── user, session, account, verification, jwks   ← Prisma (Better Auth)
└── orders, products, ...                        ← Go API
```

- **Pros:** one connection string, one bill, simpler ops, Go can JOIN auth tables if needed (e.g., "get all orders for this user's email")
- **Cons:** Prisma and Go both touch the same database — coordinate migrations so they don't conflict. Don't let Prisma `db push` overwrite Go's tables or vice versa.

### Separate databases

Better Auth gets its own database; the Go API gets its own. The JWT's `sub` (user ID) is the only link between them.

```
Neon DB 1 (auth)              Neon DB 2 (app)
├── user                      ├── orders
├── session                   ├── products
├── account                   └── ...
├── verification
└── jwks
```

- **Pros:** clean separation, each service owns its schema, no migration conflicts
- **Cons:** two connection strings, two bills, can't JOIN across databases, Go can't read user details directly (must rely on JWT claims only)

### Which to pick?

- **Small/medium projects:** shared database. One Neon project, one connection string. Just keep Prisma's `db push` away from Go's tables (Prisma only manages models defined in `schema.prisma`).
- **Large/teams:** separate databases. Each service is independently deployable and migratable.
- **The JWT bridge works either way** — the Go API never reads the auth database directly. It only verifies JWTs against the JWKS endpoint. The database question only matters if Go needs to query auth data (e.g., listing all users for an admin panel).

## License

MIT
