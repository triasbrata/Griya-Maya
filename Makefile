.PHONY: tidy build build-pdf run test lint docker docs deploy d1-migrate

# Base build: CBZ/EPUB only, no CGO.
build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

# Full build with PDF support (MuPDF via go-fitz, needs CGO + C toolchain).
build-pdf:
	CGO_ENABLED=1 go build -tags mupdf -o bin/server ./cmd/server

run:
	go run ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

vet:
	go vet ./...

# Regenerate the API spec from swag annotations (https://github.com/swaggo/swag).
# All response types live in internal packages, so --parseInternal is enough;
# --parseDependency is intentionally omitted (it drags in wazero/x/text and
# fails go list). The generated swagger/swagger.yaml is the single source of
# truth: it is embedded and served at /openapi.yaml (see internal/server/router.go).
# Run this after changing any handler's swagger comments and commit the result.
docs:
	swag init -g cmd/server/main.go -o internal/server/swagger --parseInternal

docker:
	docker build -t mihon-manga-server .

# Apply the D1 schema (requires wrangler + a created DB named "manga").
d1-migrate:
	wrangler d1 execute manga --file=migrations/0001_init.sql

deploy:
	wrangler deploy
