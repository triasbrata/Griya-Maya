.PHONY: tidy build build-pdf run run-server tunnel test cover mocks lint docker docs deploy d1-migrate

# cloudflared named tunnel that fronts the local server (config in ~/.cloudflared).
TUNNEL ?= griyamedia-tunnel

# Base build: CBZ/EPUB only, no CGO.
build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

# Full build with PDF support (MuPDF via go-fitz, needs CGO + C toolchain).
build-pdf:
	CGO_ENABLED=1 go build -tags mupdf -o bin/server ./cmd/server

# Run the server together with the cloudflared tunnel. The tunnel runs in the
# background; Ctrl-C (or the server exiting) tears it down via the trap.
run:
	@echo ">> starting cloudflared tunnel '$(TUNNEL)' in background"
	@cloudflared tunnel run $(TUNNEL) & \
		TUNNEL_PID=$$!; \
		trap 'echo; echo ">> stopping tunnel (pid $$TUNNEL_PID)"; kill $$TUNNEL_PID 2>/dev/null' EXIT INT TERM; \
		echo ">> starting server (loads .env via godotenv)"; \
		go run ./cmd/server

# Server only, without the tunnel.
run-server:
	go run ./cmd/server

# Tunnel only (foreground).
tunnel:
	cloudflared tunnel run $(TUNNEL)

tidy:
	go mod tidy

test:
	go test ./...

# Coverage across the layered packages (excludes generated mocks + infra clients).
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

# Regenerate interface mocks (mockery v2, config in .mockery.yaml).
mocks:
	mockery

vet:
	go vet ./...

# Regenerate the API spec from swag annotations (https://github.com/swaggo/swag).
# All response types live in internal packages, so --parseInternal is enough;
# --parseDependency is intentionally omitted (it drags in wazero/x/text and
# fails go list). The generated swagger/swagger.yaml is the single source of
# truth: it is embedded and served at /openapi.yaml (see internal/server/router.go).
# Run this after changing any handler's swagger comments and commit the result.
docs:
	swag init --v3.1 -g cmd/server/main.go -o internal/server/swagger --parseInternal

docker:
	docker build -t mihon-manga-server .

# Apply every D1 migration in order (requires wrangler + a created DB named
# "manga"). Migrations are forward-only; the schema ones are idempotent
# (CREATE ... IF NOT EXISTS, INSERT OR IGNORE), so a full replay provisions a
# fresh DB cleanly. Pass D1_MIGRATE_FLAGS=--remote to target the deployed DB
# (default targets the local one). NOTE: ADD COLUMN migrations are not
# re-runnable against a DB that already has them — apply a single new migration
# with `wrangler d1 execute manga --file=migrations/000N_*.sql` instead.
d1-migrate:
	@for f in $$(ls migrations/*.sql | sort); do \
		echo "==> applying $$f"; \
		wrangler d1 execute manga $(D1_MIGRATE_FLAGS) --file=$$f || exit 1; \
	done

deploy:
	wrangler deploy
