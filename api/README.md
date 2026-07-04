# Go API

JWT-protected API backend for the Better Auth Go project. Verifies Better Auth JWTs against the Next.js JWKS endpoint.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/verify` | Verify JWT from `Authorization: Bearer` header |
| GET | `/api/me` | Same as `/api/verify` — returns user info |

## Setup

```bash
cp .env.example .env
go mod download
go run .
```

The server starts on `http://localhost:8080` (or the port in `.env`).

> **Tip:** use [`air`](https://github.com/air-verse/air) for live reloading:
> ```bash
> go install github.com/air-verse/air@latest
> air
> ```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `JWKS_URL` | Better Auth JWKS endpoint | `http://localhost:3000/api/auth/jwks` |

## How JWT verification works

1. The `auth.UserFromRequest` function fetches the public key set from `JWKS_URL`
2. It parses the JWT from the request's `Authorization` header using [`lestrrat-go/jwx`](https://github.com/lestrrat-go/jwx)
3. It verifies the JWT signature against the key set (Ed25519)
4. It extracts the user ID (from `sub`), email, and name from the token claims

## Production Notes

- **JWKS caching:** this currently fetches the key set on every request. For production, use [`jwk.NewCache`](https://github.com/lestrrat-go/jwx) to cache and refresh periodically.
- **CORS:** `cors.AllowAll` is enabled for development. If you're using the Next.js proxy approach (recommended), the browser never calls Go directly and CORS isn't needed — constrain or remove it in production.
