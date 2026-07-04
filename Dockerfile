# syntax=docker/dockerfile:1

# ---- build stage ---------------------------------------------------------
# Debian-based (not Alpine) so the optional MuPDF/CGO build works cleanly.
FROM golang:1.23-bookworm AS build

WORKDIR /src

# Cache modules first.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Build with PDF support enabled. MuPDF (via go-fitz) needs CGO + a C toolchain,
# both present in the bookworm image. Drop `-tags mupdf` and set CGO_ENABLED=0
# for a smaller CBZ/EPUB-only binary.
ARG BUILD_TAGS=mupdf
ENV CGO_ENABLED=1
RUN go build -tags "${BUILD_TAGS}" -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime stage -------------------------------------------------------
FROM debian:bookworm-slim AS runtime

# MuPDF renders PDFs at runtime through go-fitz's statically-linked lib, so no
# extra runtime packages are required beyond CA certificates.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/server /usr/local/bin/server

# Cloudflare Containers route to the port your Worker advertises; default 8080.
ENV HTTP_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
