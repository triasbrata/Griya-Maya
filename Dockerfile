# syntax=docker/dockerfile:1

# ---- build stage ---------------------------------------------------------
# Alpine (musl libc). go-fitz ships a static libmupdf built for musl, selected
# via the `musl` build tag; AVIF is pure Go (wazero + embedded wasm). A CGO build
# with a C toolchain links MuPDF into the binary. The alpine minor is pinned so
# the build-time and runtime musl (and libstdc++) match.
FROM golang:1.25-alpine3.21 AS build

# build-base = gcc/g++ + musl-dev (for CGO); git for any VCS-based module fetch.
RUN apk add --no-cache build-base git

WORKDIR /src

# Cache modules first.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Build with PDF support: `mupdf` pulls in go-fitz, `musl` selects its musl
# static libs (libmupdf*_linux_*_musl.a). For a smaller, fully pure-Go
# CBZ/EPUB-only image instead, set BUILD_TAGS="" and CGO_ENABLED=0.
ARG BUILD_TAGS="mupdf musl"
ENV CGO_ENABLED=1
RUN go build -tags "${BUILD_TAGS}" -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime stage -------------------------------------------------------
# Same alpine minor as the build image so musl/libstdc++ ABIs match.
FROM alpine:3.21 AS runtime

# ca-certificates: HTTPS to R2/D1/Ory. libstdc++/libgcc: MuPDF's bundled
# third-party libs (harfbuzz is C++). AVIF needs nothing (embedded wasm).
RUN apk add --no-cache ca-certificates libstdc++ libgcc

COPY --from=build /out/server /usr/local/bin/server

# Cloudflare Containers route to the port your Worker advertises; default 8080.
ENV HTTP_ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
