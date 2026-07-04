/**
 * Fronting Worker for the Go manga server.
 *
 * Cloudflare Containers are invoked through a Worker + Durable Object. This
 * Worker forwards every request to the Go container (Hertz on :8080) and injects
 * the R2 (S3) / D1 (REST) / KV (REST) / OIDC credentials into the container's
 * environment. The Go app hosts its own embedded OpenID Provider.
 *
 * The Go app does all the work (API, conversion, docs); the Worker is just the
 * edge entrypoint. Set secrets with `wrangler secret put <NAME>`.
 */
import { Container, getContainer } from "@cloudflare/containers";

export class MangaServer extends Container {
  defaultPort = 8080;
  sleepAfter = "15m";

  // Credentials the Go process reads from its environment. These come from
  // Worker vars/secrets (see wrangler.jsonc `vars` + `wrangler secret put`).
  envVars = {
    HTTP_ADDR: ":8080",
    PUBLIC_BASE_URL: this.env.PUBLIC_BASE_URL,
    CF_ACCOUNT_ID: this.env.CF_ACCOUNT_ID,
    D1_DATABASE_ID: this.env.D1_DATABASE_ID,
    CF_API_TOKEN: this.env.CF_API_TOKEN,
    R2_BUCKET: this.env.R2_BUCKET,
    R2_ACCESS_KEY_ID: this.env.R2_ACCESS_KEY_ID,
    R2_SECRET_ACCESS_KEY: this.env.R2_SECRET_ACCESS_KEY,
    R2_PUBLIC_BASE_URL: this.env.R2_PUBLIC_BASE_URL,
    PRESIGN_TTL_SEC: this.env.PRESIGN_TTL_SEC,
    // Cloudflare KV (short-lived OIDC state).
    KV_NAMESPACE_ID: this.env.KV_NAMESPACE_ID,
    // Embedded OpenID Provider.
    OIDC_ISSUER: this.env.OIDC_ISSUER,
    OIDC_CRYPTO_KEY: this.env.OIDC_CRYPTO_KEY,
    OIDC_REQUIRED_SCOPE: this.env.OIDC_REQUIRED_SCOPE,
    OIDC_ADMIN_EMAIL: this.env.OIDC_ADMIN_EMAIL,
    OIDC_ADMIN_PASSWORD: this.env.OIDC_ADMIN_PASSWORD,
    ADMIN_REDIRECT_URIS: this.env.ADMIN_REDIRECT_URIS,
    IOS_REDIRECT_URIS: this.env.IOS_REDIRECT_URIS,
    // External-source OAuth connections: 32-byte key encrypting secrets/tokens.
    CONNECTIONS_ENC_KEY: this.env.CONNECTIONS_ENC_KEY,
  };
}

interface Env {
  MANGA_SERVER: DurableObjectNamespace;
  // Comma-separated origin allowlist for browser (admin panel) CORS. Optional;
  // falls back to the built-in defaults below when unset.
  CORS_ALLOW_ORIGINS?: string;
  [key: string]: unknown;
}

// Origins allowed to call the API from a browser. The admin panel is served
// from a different origin than this API (separate subdomain), so every
// authenticated call is preflighted; without these headers the browser blocks
// it. Overridable via the CORS_ALLOW_ORIGINS var (comma-separated).
const DEFAULT_ALLOWED_ORIGINS = [
  "https://griyamedia.brata.cloud",
  "http://localhost:3000",
];

function allowedOrigins(env: Env): string[] {
  const raw = env.CORS_ALLOW_ORIGINS;
  if (!raw) return DEFAULT_ALLOWED_ORIGINS;
  return raw
    .split(",")
    .map((o) => o.trim())
    .filter(Boolean);
}

// Build the CORS response headers for a request, echoing the Origin only when it
// is in the allowlist. Returns an empty object for non-browser / disallowed
// origins so nothing is leaked to unexpected callers.
function corsHeaders(request: Request, env: Env): Record<string, string> {
  const origin = request.headers.get("Origin");
  if (!origin || !allowedOrigins(env).includes(origin)) return {};
  return {
    "Access-Control-Allow-Origin": origin,
    "Vary": "Origin",
    "Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
    // Reflect what the browser asked for (falls back to the headers we use).
    "Access-Control-Allow-Headers":
      request.headers.get("Access-Control-Request-Headers") ||
      "Authorization, Content-Type",
    "Access-Control-Max-Age": "86400",
  };
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const cors = corsHeaders(request, env);

    // Answer the CORS preflight at the edge — the Go server registers no OPTIONS
    // routes, so letting it through would 404 the preflight.
    if (request.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: cors });
    }

    // One container instance per region is plenty for a stateless API; route by
    // a fixed key so all requests share warm instances.
    const container = getContainer(env.MANGA_SERVER, "manga-api");
    const res = await container.fetch(request);

    // Nothing to add for same-origin / disallowed origins.
    if (Object.keys(cors).length === 0) return res;

    // Response headers may be immutable; re-wrap so we can attach CORS headers.
    const headers = new Headers(res.headers);
    for (const [k, v] of Object.entries(cors)) headers.set(k, v);
    return new Response(res.body, {
      status: res.status,
      statusText: res.statusText,
      headers,
    });
  },
};
