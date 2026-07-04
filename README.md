# Mihon Manga Server

A Go manga backend for the Mihon iOS app, designed to run on **Cloudflare
Containers** with **R2** (object storage) and **D1** (catalog metadata). It
serves a JSON API that mirrors the app's `SourceRuntime` contract and converts
**CBZ / EPUB / PDF** archives into **AVIF** page images stored back in R2.

- **HTTP:** [Hertz](https://github.com/cloudwego/hertz) (ByteDance/CloudWeGo)
- **DI:** [Uber FX](https://github.com/uber-go/fx)
- **Layers:** `handler → service → repository (D1 / R2)`
- **Docs:** OpenAPI 3 served at `/docs` (Swagger UI) and `/openapi.yaml`
- **Auth:** Ory OAuth2 (token introspection) + Dynamic Client Registration
  (RFC 7591/7592) proxied to Ory Hydra

## Why a container (not a pure Worker)?

Hertz is a native Go framework (netpoll/syscalls) and cannot run on the V8/WASM
Workers runtime. PDF rendering and AVIF encoding are also too heavy for a
Worker's 128 MB / CPU limits. So the Go app runs in a **Cloudflare Container**,
fronted by a thin Worker (`worker/index.ts`) that routes edge traffic to it and
injects credentials.

```
App iOS ──HTTP JSON──► Worker (edge) ──► Go container (Hertz)
                                          ├─ handler → service → repo
                                          ├─ D1  (REST API)  ← catalog
                                          └─ R2  (S3 API)    ← archives + AVIF pages
                    Ory Hydra ◄── OAuth2 introspection + DCR
```

## Layout

```
cmd/server/            main.go (fx bootstrap)
internal/
  config/              env-driven configuration
  domain/              entities (mirror the app's SourceRuntime DTOs)
  repository/d1/        D1 REST client + manga/job repos
  repository/r2/        R2 (S3) object store
  convert/             CBZ/EPUB/PDF extraction + AVIF encode
  service/             business logic (ports + implementations)
  handler/             Hertz HTTP handlers (+ swag annotations)
  auth/                Ory introspection middleware + DCR proxy
  server/              Hertz engine, routing, OpenAPI, lifecycle
  app/                 fx module wiring it all together
migrations/            D1 schema
worker/                fronting Cloudflare Worker (TS)
Dockerfile             container image (optional MuPDF/PDF support)
wrangler.jsonc         Cloudflare Containers config
```

## API

| Method | Path                                   | Auth   | Purpose                          |
|--------|----------------------------------------|--------|----------------------------------|
| GET    | `/healthz`                             | –      | Liveness                         |
| GET    | `/docs`, `/openapi.yaml`               | –      | API documentation                |
| GET    | `/v1/sources/{sourceId}/popular`       | –      | Popular list (+ filters)         |
| GET    | `/v1/sources/{sourceId}/latest`        | –      | Latest list (+ filters)          |
| GET    | `/v1/sources/{sourceId}/search?q=`     | –      | Search (+ filters)               |
| GET    | `/v1/sources/{sourceId}/genres`        | –      | Filterable genres (`{slug,name}`)|
| GET    | `/v1/manga/{id}`                       | –      | Details                          |
| GET    | `/v1/manga/{id}/chapters`              | –      | Chapter list                     |
| GET    | `/v1/chapters/{id}/pages`              | –      | Page list (AVIF URLs)            |
| GET    | `/v1/image?key=`                       | –      | Proxy an AVIF object from R2      |
| POST   | `/v1/convert/upload`                   | Bearer | Upload CBZ/EPUB/PDF to R2         |
| POST   | `/v1/convert`                          | Bearer | Convert an R2 archive → AVIF      |
| GET    | `/v1/convert/jobs/{id}`                | Bearer | Job status                       |
| POST   | `/connect/register`                    | –      | Register OAuth2 client (7591)    |
| GET/PUT/DELETE | `/connect/register/{id}`       | RegTok | Manage client (7592)             |

### Catalog filters

`popular`, `latest`, and `search` accept the same filter vocabulary, mirroring the
app's `SourceFilterValue`. All are optional; omit them for the plain feed.

| Param          | Values                                        | Notes                                                            |
|----------------|-----------------------------------------------|------------------------------------------------------------------|
| `sort`         | `popular` `latest` `updated` `rating` `title` | `rating` falls back to `popularity` (no rating column).          |
| `order`        | `asc` `desc`                                  | Mirrors `.orderAscending`. Default `desc`.                       |
| `type`         | e.g. `manga` `manhwa` `manhua` (repeatable)   | No type column — matched against `genres`.                       |
| `genre`        | genre **slug** (repeatable)                   | Include. Slugs come from `/genres`.                              |
| `genreExclude` | genre **slug** (repeatable)                   | Exclude.                                                         |
| `genreMode`    | `or` `and`                                    | Combine `genre` includes. Default `or`.                          |

Repeatable params accept both forms: `?genre=action&genre=comedy` or `?genre=action,comedy`.
Genre matching is slug/name-insensitive (`Martial Arts` ≡ `martial-arts`).

Example: `/v1/sources/acme/search?q=cat&sort=title&order=asc&genre=action,comedy&genreExclude=ecchi&genreMode=and`

### iOS field mapping

The JSON mirrors the app's `SourceRuntime` DTOs, with a few key renames the
app-side `mihonServer` engine maps on decode:

| Server field (JSON)      | App DTO field                       | Note                                             |
|--------------------------|-------------------------------------|--------------------------------------------------|
| `Manga.description`      | `Manga.summary`                     | Rename.                                           |
| `Manga.status` (enum)    | `Manga.statusText` (free string)    | `ongoing`/`completed`/… → display text.          |
| `Manga.coverUrl`         | `Manga.coverURL`                    | Casing only.                                      |
| _(none)_                 | `Manga.coverHexes`                  | Placeholder palette derived client-side.         |
| `Chapter.name`           | `Chapter.title`                     | Rename.                                           |
| `Chapter.dateUpload`     | `Chapter.releaseDate`               | RFC3339 → `Date`.                                 |
| `Page.imageUrl`          | `ReaderPage.remoteURL`              | Plus `assetKind = .image`.                        |
| `GenreTag.slug`          | `GenreTag.slug` (`id` = slug)       | 1:1.                                              |
| `MangaPage.hasNext`      | `SourceFeedResult.reachedEnd`       | `reachedEnd = !hasNext` (or empty page).         |

## Conversion pipeline

`POST /v1/convert` with `{ "sourceKey": "uploads/<id>/vol1.cbz", "chapterId": "..." }`:

1. Download the archive from R2.
2. Detect format (hint → extension → magic bytes).
3. Extract pages in reading order:
   - **CBZ** — images from the ZIP, natural-sorted.
   - **EPUB** — one image per spine item (OPF), fallback to all embedded images.
   - **PDF** — MuPDF renders each page (requires the `mupdf` build tag).
4. Decode → downscale to `AVIF_MAX_EDGE` → encode AVIF (parallel workers).
5. Upload `page-NNNN.avif` under `outputPrefix` in R2; record pages in D1.

> **PDF note:** the default build supports CBZ/EPUB only. PDF rendering needs
> native MuPDF, so build with `-tags mupdf` (the Dockerfile does this by default).

## Local development

```bash
cp .env.example .env      # fill in CF/R2/Ory credentials
make tidy
make run                  # CBZ/EPUB only
# or, with PDF support (needs a C toolchain):
make build-pdf && ./bin/server
```

Open http://localhost:8080/docs.

## Deploy to Cloudflare

```bash
# 1. Create resources
wrangler r2 bucket create manga
wrangler d1 create manga                 # put the id in wrangler.jsonc
make d1-migrate                          # apply migrations/0001_init.sql

# 2. Secrets (injected into the container by worker/index.ts)
wrangler secret put CF_API_TOKEN
wrangler secret put R2_ACCESS_KEY_ID
wrangler secret put R2_SECRET_ACCESS_KEY
wrangler secret put ORY_INTROSPECT_AUTH  # optional

# 3. Ship (builds the Dockerfile, pushes the image, deploys the Worker)
wrangler deploy
```

## Wiring into the iOS app

Add a source to the app's `repo.json` pointing at this server with a new
`engineFamily` (e.g. `mihonServer`) whose `baseUrl` is the Worker URL. The
endpoints above already match the app's `SourceRuntime` methods
(popular/latest/search → `[Manga]`, details, chapters, pages), so the app-side
engine is a thin JSON client.

## Ory setup (OAuth2 + DCR)

Point `ORY_ISSUER_URL` / `ORY_ADMIN_URL` at your Hydra (self-hosted or Ory
Network). Enable Dynamic Client Registration so `/connect/register` works.
Protected routes require a Bearer access token with `ORY_REQUIRED_SCOPE`
(default `manga.write`), validated via Hydra's introspection endpoint.
