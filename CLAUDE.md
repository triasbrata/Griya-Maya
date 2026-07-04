# CLAUDE.md — griyamedia-server (Mihon Manga Server)

Guidance for AI agents (and humans) working in this repo. Read this first; it is
designed so you can locate code **without grepping** and follow the project's
conventions on the first try.

---

## 0. Non-negotiable workflow rules

Follow these on **every** task, no exceptions:

1. **Log every change.** After any code/config change, append an entry to
   `changes.log` at the repo root (newest at the bottom). Format:

   ```
   ## YYYY-MM-DD — <short title>
   - What: <what changed, 1-3 bullets>
   - Why: <reason / linked plan or issue>
   - Files: <key files touched>
   - Tests: <what was added/updated + result, e.g. "go test ./... → 107 pass">
   ```

   Use the current date (the environment provides it). One entry per logical
   task, not per file.

2. **Keep coverage above 80%.** The business-logic layers must stay `≥ 80%`
   statement coverage: `internal/domain`, `internal/service`, `internal/handler`,
   `internal/repository/d1`. Any new exported behavior in these packages ships
   **with tests in the same change**. Check before finishing:

   ```
   make cover                 # total across layered packages
   go test ./internal/service/... -cover   # per-package spot check
   ```

   Infra/adapter packages are exempt from the 80% bar because they wrap external
   SDKs/CGO and are covered by integration, not unit tests: `internal/convert`
   (AVIF/PDF encoders), `internal/repository/{r2,kv}`, `internal/repository/d1/client.go`,
   `internal/oidc`, `internal/server`, `internal/app`, `internal/config`,
   `cmd/server`. Don't lower coverage in the four gated packages to make a change
   pass — add the test instead.

   > Current baseline (2026-07): domain 100%, handler 79.7%, d1 75.1%,
   > service 71.4%. Treat sub-80% packages as **debt to pay down**, not a license
   > to add more untested code. New code must push a gated package toward/over 80%.

3. **Regenerate what's generated.** Never hand-edit generated files:
   - Changed an interface in a `ports.go`? Run `make mocks` (mockery, config in
     `.mockery.yaml`). Mocks live in each package's `mocks/` subdir.
   - Changed a handler's `// @…` swagger annotation? Run `make docs` (swag). The
     embedded spec (`internal/server/swagger/*`) is the source of truth served at
     `/openapi.yaml` and `/docs`. Commit the regenerated files.

4. **Verify before declaring done:** `go build ./... && go vet ./... && go test ./...`.

---

## 1. What this is

A Go backend for the Mihon iOS reader app. It runs inside a **Cloudflare
Container** (fronted by a thin Worker), serves a JSON API mirroring the app's
`SourceRuntime` contract, converts **CBZ/EPUB/PDF** archives into **AVIF** pages
stored in **R2**, and hosts its **own embedded OpenID Provider** for auth.

Data plane: catalog metadata in **D1**, page bytes in **R2**, short-lived OIDC
state in **KV**. Page bytes are served to clients as **short-lived presigned R2
URLs** (direct client→R2, no proxy) gated behind a `manga.read` token.

---

## 2. Tech stack & how to use it

| Concern | Choice | How it's used here |
|---|---|---|
| HTTP framework | **Hertz** (cloudwego) | `internal/server/router.go` builds the engine; handlers are `func(ctx, *app.RequestContext)`. Native Go — cannot run on the Workers V8 runtime, hence the container. |
| Dependency injection | **Uber FX** | `internal/app/module.go` is the whole graph. Providers bind concrete types to ports via `fx.Annotate(New, fx.As(new(Port)))`. |
| Object storage | **R2** via **aws-sdk-go-v2 S3** | `internal/repository/r2/store.go`. R2 needs `UsePathStyle` + `region "auto"` + account endpoint. |
| Catalog DB | **D1** via REST | `internal/repository/d1/client.go` speaks the D1 HTTP API (no SQL driver). Values arrive as `string`/`float64`/`nil` — decode with the helpers in `internal/oidc/util.go` / repo code. |
| Ephemeral state | **KV** via REST | `internal/repository/kv/client.go`. TTL-backed OIDC auth requests/codes/tokens. |
| Auth | **zitadel/oidc/v3** embedded OP | `internal/oidc/*`. RS256 JWT access tokens, verified locally against the OP's JWKS. Scopes: `manga.read` (reader), `manga.write` (ingest/convert). |
| Image encode | **gen2brain/avif** | `internal/convert/avif.go`. Tuned by `AVIF_*` env. |
| PDF | **gen2brain/go-fitz** (MuPDF) | `internal/convert/pdf_mupdf.go`, behind build tag `mupdf` (**needs CGO**). `pdf_stub.go` is the no-CGO fallback. |
| API docs | **swaggo/swag v2** | Annotations on handlers → `make docs` → embedded OpenAPI 3.1. |
| Mocks | **mockery v2** | `make mocks`, config `.mockery.yaml`. |
| Tests | **testify** | `assert`/`require` + generated mocks with `.EXPECT()`. |
| Edge | **Cloudflare Worker** | `worker/index.ts` forwards requests to the container and injects env/secrets. Config in `wrangler.jsonc`. |

### Everyday commands (Makefile)

```
make build        # CGO-off binary (CBZ/EPUB only), -> bin/server
make build-pdf    # CGO-on binary with PDF (tag mupdf, needs C toolchain)
make run-server   # go run ./cmd/server (loads .env via godotenv)
make run          # server + cloudflared tunnel
make test         # go test ./...
make cover        # coverage across layered packages
make mocks        # regenerate mockery mocks
make docs         # regenerate OpenAPI from swag annotations
make d1-migrate   # apply migrations/0001_init.sql via wrangler
make deploy       # wrangler deploy
```

Local dev: copy `.env.example` → `.env`, fill creds, `make run-server`.
`.env` is auto-loaded; already-set env vars win.

---

## 3. Architecture & folder layout

Strict layering — dependencies point **downward only**:

```
handler  →  service  →  repository (d1 / r2 / kv)  +  convert
   ↑            ↑
 ports.go    ports.go        (interfaces live in the CONSUMER package)
```

- **Ports are defined by the consumer.** `internal/handler/ports.go` declares the
  service interfaces handlers need; `internal/service/ports.go` declares the
  repository/store/converter interfaces services need. Concrete impls are bound to
  these ports in `internal/app/module.go`. This is what makes everything mockable.
- **Domain has no dependencies.** `internal/domain` is pure types, imported by all.
- Handlers do HTTP only (parse → call service → write). Business rules live in
  services. Storage lives in repositories. Keep it that way.

### File map (find code without grep)

**Entry / wiring**
- `cmd/server/main.go` — `main()`: `fx.New(app.Module).Run()`; top-level swagger `@securityDefinitions`.
- `internal/app/module.go` — the entire FX dependency graph; port→impl bindings; small `newXService` constructors that inject config.
- `internal/config/config.go` — `Load()` reads all env into typed `Config` structs; defaults + validation.

**HTTP layer — `internal/server`**
- `router.go` — all route registration; middleware groups (public catalog, `manga.read`-gated reader, `manga.write`-gated management + ingest, DCR); serves `/docs` + `/openapi.yaml`.
- `lifecycle.go` — FX start/stop hooks for the Hertz server.
- `swagger/` — **generated** OpenAPI (`docs.go`, `swagger.json`, `swagger.yaml`). Do not edit by hand.

**Handlers — `internal/handler`** (Hertz `func(ctx, *app.RequestContext)`)
- `ports.go` — `MediaService`/`TaxonomyService`/`ConvertService`/`VideoService`/`NovelService` interfaces (what handlers depend on).
- `response.go` — `ErrorResponse`, `writeError`, status mapping, `parseCatalogFilter` (type/genre/category query params).
- `health_handler.go` — `/healthz`.
- `media_handler.go` — catalog (`popular/latest/search/genres/categories/details/chapters`), reader (`pages`), gated `image` proxy, and media/chapter CRUD.
- `taxonomy_handler.go` — genre/category/author/artist CRUD via `/v1/taxonomies/{kind}`.
- `convert_handler.go` — `upload` / `convert` / `jobs/:id`.
- `video_handler.go` — HLS `upload` / `register` / `stream`.
- `novel_handler.go` — novel text `register`.
- `mocks/` — **generated** service mocks for handler tests.

**Business logic — `internal/service`**
- `ports.go` — `MediaRepository` (reads + media/chapter writes + taxonomy CRUD), `JobRepository`, `ObjectStore`, `ArchiveConverter` interfaces.
- `media_service.go` — catalog/reader logic; mints presigned page URLs (`pageURL`), inlines novels, builds stream URLs; media + chapter CRUD (validation, id generation).
- `taxonomy_service.go` — genre/category/author/artist management (kind-parametrized).
- `convert_service.go` — orchestrates archive → AVIF → R2 → D1 pages.
- `video_service.go` — registers uploaded HLS bundles as chapter pages.
- `novel_service.go` — stores/inlines novel text.
- `mocks/` — **generated** repository/store mocks.

**Persistence — `internal/repository`**
- `d1/client.go` — D1 REST client + `Querier` usage; row-value decoding.
- `d1/media_repo.go` — catalog reads (list/search/genres/categories/get/chapters/pages), filter→SQL builder (type column + genre/category EXISTS joins), media/chapter writes, and taxonomy CRUD (`taxTableFor` routes the 4 kinds). Taxonomies reassembled per-row via `group_concat(...,char(31))` subqueries.
- `d1/job_repo.go` — convert-job lifecycle + `ReplacePages`.
- `d1/ports.go` — `Querier` interface (mocked for repo tests).
- `d1/mocks/` — **generated**.
- `kv/client.go` — KV REST client (TTL puts/gets) for OIDC state.
- `r2/store.go` — R2 S3 wrapper: `Get`/`Put`/`PublicURL`/`PresignGet`.

**Auth — `internal/oidc`** (embedded OpenID Provider)
- `provider.go` — builds the zitadel OP + composite HTTP handler; `SupportedScopes`.
- `storage.go` — implements zitadel `op.Storage` over D1+KV; signing-key load, admin/client seeding on boot.
- `middleware.go` — Bearer JWT verify + scope gate: `Middleware()` (provider-wide scope) and `MiddlewareScope(scope)`.
- `seed.go` — seeds static public PKCE clients (`admin-web`, `mihon-ios`).
- `user.go` — user store (login lookup).
- `client.go` / `models.go` — OIDC client + auth-request/token models.
- `dcr.go` — Dynamic Client Registration (RFC 7591/7592).
- `login.go` / `templates.go` — htmx login/consent UI.
- `util.go` — scope consts (`ScopeMangaRead`/`ScopeMangaWrite`), argon2id hashing, D1 value/JSON helpers, `randToken`.

**Domain — `internal/domain`** (pure types, no deps)
- `manga.go` — the unified **`Media`** entity (+ `MediaType` manga|video|novel), `Chapter` (`MediaID`), `Page`, `StoredPage`, `MediaPage`, `CatalogFilter` (type/genre/category filters), `Taxonomy`/`TaxonomyKind`, `MediaWriteRequest`/`ChapterWriteRequest`/`TaxonomyWriteRequest`, page-kind consts.
- `convert.go` — `ConvertJob`, `ConvertRequest` (`MediaID`), `ArchiveFormat`, `ConvertStatus`.
- `novel.go` — novel request/types. `video.go` — video request/types.

**Conversion engine — `internal/convert`** (infra; heavy, partly CGO)
- `converter.go` — `Converter` orchestrator (format → pages).
- `cbz.go` / `epub.go` / `extract.go` — archive extraction.
- `avif.go` — AVIF encode/downscale.
- `pdf_mupdf.go` (tag `mupdf`, CGO) / `pdf_stub.go` (fallback).

**Edge & ops**
- `worker/index.ts` — fronting Worker; the `envVars` map is the **allow-list of env passed to the container** — add new server env here too.
- `wrangler.jsonc` — Worker vars/bindings (KV, container). Secrets via `wrangler secret put`.
- `migrations/*.sql` — D1 schema (`0001_init`, `0002_oidc`, `0003_video`, `0004_media_normalize` — renames `manga`→`media`, normalizes genre/category/author/artist into tables + join tables).
- `Dockerfile` — container image. `Makefile` — all tasks. `.env.example` — every env var, documented.

---

## 4. How to develop new things (recipes)

### Add a new API endpoint
1. **Domain** (if new shapes): add types in `internal/domain`.
2. **Service**: add the method to the relevant `*_service.go`; if it needs a new
   dependency, add it to that package's `ports.go`. Write a table/mock test in
   `*_service_test.go`.
3. **Port + mock**: if you added to `service/ports.go` or `handler/ports.go`, run
   `make mocks`.
4. **Handler**: add the handler method with `// @…` swagger annotations (copy the
   style of neighbors; include `@Security BearerAuth` + `401/403` if gated). Write
   a handler test with the generated service mock.
5. **Route**: register it in `internal/server/router.go` under the correct group
   (public / `manga.read` / `manga.write`).
6. `make docs`, then `go build ./... && go vet ./... && make test && make cover`.
7. Append to `changes.log`.

### Add a new storage operation (R2/D1/KV)
1. Implement on the concrete type (`r2/store.go`, `d1/*_repo.go`, `kv/client.go`).
2. Add it to the consumer port (`service/ports.go` or `d1/ports.go`), `make mocks`.
3. Use it from the service; test the service against the mock.

### Add config / env var
`internal/config/config.go` (struct field + `env`/`envInt` default) **and**
`.env.example` **and** `worker/index.ts` `envVars` **and** (if a non-secret Worker
var) `wrangler.jsonc`. Missing any of these means the value won't reach the
container in production.

### Add a DB migration
New file `migrations/000N_desc.sql`. Apply locally with `wrangler d1 execute manga
--file=migrations/000N_desc.sql`. Keep migrations forward-only and idempotent
where feasible.

### Add a new archive/page format
Extend `internal/convert` (new `*.go` + wire into `converter.go`). If it needs a C
lib, gate it behind a build tag with a stub fallback like `pdf_mupdf.go` / `pdf_stub.go`.

---

## 5. Conventions

- **Errors:** wrap with context (`fmt.Errorf("r2 get %q: %w", key, err)`); handlers
  translate to HTTP via `writeError`.
- **Comments** explain *why*, not *what*; match the density of surrounding code.
- **Tests** are `package foo_test` (black-box) using generated mocks + testify
  `require`/`assert`; name them `TestType_Method_Scenario`.
- **Never** make R2 public: keep `R2_PUBLIC_BASE_URL` empty; page bytes flow via
  presigned URLs gated by `manga.read`.
- **Don't** route page bytes back through the container proxy — that path
  (`/v1/image`) is retired behind the read gate.

---

## 6. Sibling repos

This server is one of three repos in the GriyaMedia system. The other two live
alongside it as siblings of this repo's directory:

| Repo | Path | What it is |
|---|---|---|
| **griyamedia-admin** | `../griyamedia-admin/` | Admin panel (TanStack Start → Cloudflare Workers). Authenticates against this server's embedded OP (`authorization_code` + PKCE, public client `admin-web`) and drives the protected media/video/novel/convert API. |
| **mihon / GriyaMedia IOS** | `../mihon/` | The iOS reader app (Mihon fork). Consumes this server's `SourceRuntime`-shaped API as its content source. |

**Rule: when a task requires checking, cross-referencing, or changing anything in
a sibling repo, spawn a sub-agent to read that repo's own `CLAUDE.md` first** —
don't grep it blind from here. Each sibling has its own conventions, file map, and
workflow rules that its `CLAUDE.md` documents. Launch a `general-purpose` (or
`Explore`) agent pointed at the sibling path and have it read
`../griyamedia-admin/CLAUDE.md` or `../mihon/CLAUDE.md` before answering. This
keeps their guidance authoritative and avoids polluting this repo's context with
their file trees. When several siblings are involved, spawn the agents in
parallel.
