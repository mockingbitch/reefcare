# ReefCare developer commands. Go tooling runs locally when `go` is installed,
# otherwise inside the official golang container (same version as the Dockerfile).

GO_IMAGE        := golang:1.27
GOLANGCI_IMAGE  := golangci/golangci-lint:v2.15.0
SQLC_IMAGE      := sqlc/sqlc:1.31.1
SQLC            := docker run --rm -v $(CURDIR):/src -w /src/db $(SQLC_IMAGE)
COMPOSE_NETWORK := reefcare_default
DOCKER_GO_RUN   := docker run --rm -v $(CURDIR):/src -v reefcare-gomod:/go/pkg/mod \
                   -v reefcare-gocache:/root/.cache/go-build -w /src

ifeq ($(shell command -v go 2>/dev/null),)
GO                := $(DOCKER_GO_RUN) $(GO_IMAGE) go
GO_INTEGRATION    := $(DOCKER_GO_RUN) --network $(COMPOSE_NETWORK) \
                     -e TEST_DATABASE_URL=postgres://reefcare:reefcare@db:5432/reefcare_test?sslmode=disable \
                     $(GO_IMAGE) go
GOFMT             := $(DOCKER_GO_RUN) $(GO_IMAGE) gofmt
else
GO                := go
GO_INTEGRATION    := TEST_DATABASE_URL=postgres://reefcare:reefcare@localhost:5432/reefcare_test?sslmode=disable go
GOFMT             := gofmt
endif

.PHONY: help fmt fmt-check vet lint test test-integration build vuln check sqlc sqlc-check \
        up down logs migrate-up migrate-down migrate-status migration

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-18s %s\n",$$1,$$2}'

fmt: ## Format Go code
	$(GOFMT) -w .

fmt-check: ## Fail if any Go file is not gofmt-formatted
	@out=$$($(GOFMT) -l .); if [ -n "$$out" ]; then echo "unformatted files:"; echo "$$out"; exit 1; fi

vet: ## go vet (including integration-tagged files)
	$(GO) vet -tags=integration ./...

lint: vet ## golangci-lint
	docker run --rm -v $(CURDIR):/src -v reefcare-gomod:/go/pkg/mod -w /src $(GOLANGCI_IMAGE) golangci-lint run ./...

test: ## Unit tests with the race detector
	$(GO) test -race -count=1 ./...

test-integration: ## Integration tests against the compose PostgreSQL (run `make up` first)
	$(GO_INTEGRATION) test -race -count=1 -tags=integration ./...

build: ## Build the API binary into bin/
	$(GO) build -trimpath -o bin/api ./cmd/api

vuln: ## Scan dependencies for known vulnerabilities
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

sqlc: ## Regenerate type-safe query code from db/queries into internal/db/dbq
	$(SQLC) generate

sqlc-check: ## Fail if generated query code is out of date
	$(SQLC) diff

check: fmt-check sqlc-check lint test ## Fast local verification

up: ## Start the dev stack (db, mailpit, migrate, api)
	docker compose up -d --build

down: ## Stop the dev stack (keeps data volumes)
	docker compose down

logs: ## Follow API logs
	docker compose logs -f api

migrate-up: ## Apply pending migrations to the dev database
	docker compose run --rm --build migrate migrate up

migrate-down: ## Roll back the latest migration on the dev database
	docker compose run --rm --build migrate migrate down

migrate-status: ## Show migration status of the dev database
	docker compose run --rm --build migrate migrate status

migration: ## Create a new sequential migration: make migration name=create_users
	@test -n "$(name)" || (echo "usage: make migration name=<snake_case>"; exit 1)
	@last=$$(ls db/migrations/*.sql 2>/dev/null | sed -E 's|.*/0*([0-9]+)_.*|\1|' | sort -n | tail -1); \
	next=$$(printf "%05d" $$(( $${last:-0} + 1 ))); \
	file=db/migrations/$${next}_$(name).sql; \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > $$file; echo "created $$file"
