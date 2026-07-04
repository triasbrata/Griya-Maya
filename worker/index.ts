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
    // Browser CORS allowlist for the admin panel (comma-separated origins).
    CORS_ALLOW_ORIGINS: this.env.CORS_ALLOW_ORIGINS,
    // Cloudflare Queue (UUID) backing the async cover-image mirror. Empty
    // disables mirroring (covers stay their original URLs). The container
    // produces/pull-consumes via the Queues REST API using CF_ACCOUNT_ID +
    // CF_API_TOKEN (token needs Queues read+write).
    COVER_QUEUE_ID: this.env.COVER_QUEUE_ID,
  };
}

interface Env {
  MANGA_SERVER: DurableObjectNamespace;
  [key: string]: unknown;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    // One container instance per region is plenty for a stateless API; route by
    // a fixed key so all requests share warm instances. CORS (incl. the OPTIONS
    // preflight) is handled inside the Go server so it applies to both this
    // Worker path and the direct cloudflared-tunnel path.
    const container = getContainer(env.MANGA_SERVER, "manga-api");
    return container.fetch(request);
  },
};
