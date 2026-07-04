# Mihon Manga Admin (TanStack Start)

Admin panel for `manga-server`, hosted on Cloudflare. It authenticates against
the **embedded OpenID Provider** in manga-server using `authorization_code` +
PKCE (public client `admin-web`), then calls the protected manga API.

## Stack
- [TanStack Start](https://tanstack.com/start) (React, Vite)
- [`oidc-client-ts`](https://github.com/authts/oidc-client-ts) for PKCE login
- Deployed to Cloudflare Workers

## Auth flow
1. `Sign in` → `signinRedirect()` → manga-server `/authorize` (login + consent, htmx UI).
2. Redirect back to `/auth/callback` → PKCE code exchange at `/oauth/token`.
3. Access token (JWT) attached as `Bearer` on API calls; refresh via silent renew.

The client (`admin-web`, its redirect URI, scopes) is seeded in manga-server's
D1 on first boot. Make sure this app's origin is in the server's
`ADMIN_REDIRECT_URIS` (e.g. `https://admin.example.com/auth/callback`).

## Develop
```bash
cp .env.example .env      # point VITE_OIDC_ISSUER / VITE_API_BASE at manga-server
npm install
npm run dev               # http://localhost:3000
```

## Deploy to Cloudflare
```bash
DEPLOY_TARGET=cloudflare-module npm run build
npm run deploy            # wrangler
```

## Env
| Var | Meaning |
|-----|---------|
| `VITE_OIDC_ISSUER` | manga-server base URL (the IdP issuer) |
| `VITE_OIDC_CLIENT_ID` | OAuth2 client id (default `admin-web`) |
| `VITE_OIDC_SCOPE` | requested scopes |
| `VITE_API_BASE` | manga-server API base URL |
