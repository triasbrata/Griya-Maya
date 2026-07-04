# API migration: `manga` → unified `media` (handoff for the iOS agent)

**Status:** breaking. The server API was fully replaced (no `/v1/manga*` aliases).
The Mihon iOS client (`MihonServerSourceEngine` and the reader/catalog models)
must be updated in lockstep. This note lists every change with before/after
examples so the iOS side can be migrated without reading the server code.

> Auth is unchanged from the R2-presign work: catalog browse is public, the page
> list (`/v1/chapters/{id}/pages`) requires a `manga.read` bearer, and all the new
> **management** endpoints require a `manga.write` bearer. Page bytes are still
> short-lived presigned R2 URLs (decouple cache keys from the signature — see the
> earlier R2 handoff).

---

## 1. Concept change: one `media` entity for manga / video / novel

There is now a single catalog entity, **`media`**, discriminated by a `type`
field (`"manga" | "video" | "novel"`). Manga, video, and novel all share the
same shape and the same endpoints — the client picks the reader from `type` (and
still from each `Page.type`). Genres, categories, authors, and artists are
**normalized** and returned as **string arrays**.

## 2. Renamed paths

| Before | After |
|---|---|
| `GET /v1/manga/{id}` | `GET /v1/media/{id}` |
| `GET /v1/manga/{id}/chapters` | `GET /v1/media/{id}/chapters` |
| `GET /v1/sources/{sourceId}/popular` | *(unchanged path)* — items are now `media` |
| `GET /v1/sources/{sourceId}/latest` | *(unchanged)* |
| `GET /v1/sources/{sourceId}/search` | *(unchanged)* |
| `GET /v1/sources/{sourceId}/genres` | *(unchanged)* — see shape change below |
| — | `GET /v1/sources/{sourceId}/categories` *(new)* |
| `GET /v1/chapters/{id}/pages` | *(unchanged)* — still `manga.read`-gated |
| `GET /v1/image` | *(unchanged)* — still gated, prefer presigned URLs |

## 3. Renamed / changed JSON fields

### Browse item & detail (`media`)

Before (`manga`):
```json
{
  "id": "m1", "sourceId": "src", "url": "…", "title": "One Piece",
  "coverUrl": "…", "author": "Oda", "artist": "Oda",
  "description": "…", "genres": ["Action", "Adventure"],
  "status": "ongoing", "updatedAt": "…"
}
```

After (`media`):
```json
{
  "id": "m1", "sourceId": "src", "type": "manga", "url": "…", "title": "One Piece",
  "coverUrl": "…", "description": "…",
  "genres": ["Action", "Adventure"],
  "categories": ["Shonen"],
  "authors": ["Oda"],
  "artists": ["Oda"],
  "status": "ongoing", "updatedAt": "…"
}
```

Changes:
- **added** `type` (`"manga" | "video" | "novel"`).
- `author: string` → **`authors: string[]`**.
- `artist: string` → **`artists: string[]`**.
- **added** `categories: string[]`.
- `genres: string[]` unchanged.

Paginated browse (`MediaPage`) shape is unchanged: `{ items: Media[], hasNext, page }`.

### Chapter

- `mangaId` → **`mediaId`**. Everything else (`id`, `url`, `name`, `number`,
  `scanlator`, `dateUpload`, `format`) is unchanged.

### Genres / categories list

`GET /v1/sources/{id}/genres` (and the new `…/categories`) now return objects
with an `id`:
```json
[{ "id": "g1", "slug": "action", "name": "Action" }]
```
(Previously `{ "slug", "name" }` only — `id` is additive; existing slug-based
filtering still works.)

### Page (reader) — unchanged

`Page` is unchanged: `{ index, imageUrl, type?, body?, width?, height? }`. Video
pages carry `type:"video"` + an `.m3u8` `imageUrl`; novel pages carry
`type:"novel"` + `body`.

## 4. Changed filter query params

On the browse/search endpoints:
- **`type`** now filters the media kind directly: `type=manga|video|novel`
  (repeatable / comma-joinable). **Breaking:** previously `type` matched content
  tags like `manhwa`/`manhua` against genres — those now belong to `category`.
- **`category`** / **`categoryExclude`** — new, filter the category taxonomy by
  slug (repeatable), mirroring `genre` / `genreExclude`.
- `genre`, `genreExclude`, `genreMode`, `sort`, `order`, `page` — unchanged.

Example: novels tagged webtoon, excluding ecchi:
`GET /v1/sources/src/search?q=…&type=novel&category=webtoon&genreExclude=ecchi`

## 5. New management endpoints (require `manga.write`)

The client only needs these if it manages the catalog (admin flows). Readers can
ignore them.

- Media: `POST /v1/media`, `PUT /v1/media/{id}`, `DELETE /v1/media/{id}`.
  Body = `MediaWriteRequest`:
  ```json
  { "sourceId":"src", "type":"video", "url":"…", "title":"…",
    "coverUrl":"…", "description":"…", "status":"ongoing",
    "genres":["Action"], "categories":["Webtoon"],
    "authors":["Oda"], "artists":["Oda"] }
  ```
  Taxonomies are sent as **names**; the server upserts them and returns the stored
  media with normalized arrays. `POST` → `201`, `DELETE` → `204` (cascades
  chapters/pages/links).
- Chapters: `POST /v1/media/{id}/chapters` (path id is authoritative),
  `PUT /v1/chapters/{id}`, `DELETE /v1/chapters/{id}`.
  Body = `ChapterWriteRequest`: `{ "mediaId","url","name","number","scanlator","dateUpload","format" }`.
- Taxonomies: `GET|POST /v1/taxonomies/{kind}`, `PUT|DELETE /v1/taxonomies/{kind}/{id}`,
  where `{kind}` ∈ `genres | categories | authors | artists`. Create/update body:
  `{ "name":"Action" }`. Returns `{ id, slug?, name }` (`slug` only for
  genres/categories).

## 6. Suggested iOS work

- Rename the model `Manga`→`Media` (or keep the Swift type name, just remap JSON
  keys) and add `type`; change `author`/`artist` (String) → `authors`/`artists`
  (`[String]`); add `categories: [String]`.
- Rename `mangaId`→`mediaId` on the chapter model and any request builders.
- Update the detail/chapters URLs `/v1/manga/…` → `/v1/media/…`.
- If the app exposes a content-type filter (manga/manhwa/manhua), split it: media
  kind → `type`; content categories → `category`.
- Genre picker: `genres`/`categories` list responses now include `id` (additive).
- (Optional) build the management UI against the new `manga.write` endpoints.
