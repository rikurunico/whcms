# WHCMS — root task runner.
#
# Single entry point for the day-to-day dev/test commands. The heavy lifting
# still lives in scripts/ and backend/Makefile; these targets just give them a
# consistent, discoverable interface.
#
#   make up             start the full local stack (mockserver+api+seed+worker+frontend)
#   make down           stop the local stack
#   make tunnel         start a cloudflared tunnel to the API and list this project's webhooks
#   make lint           gofmt + golangci-lint + mockserver vet + e2e import guard (what CI lints)
#   make test-backend   backend unit tests + coverage report (enforces the >=90% gate)
#   make test-frontend  Playwright E2E tests against the running stack
#   make hooks          install the repo git hooks (pre-push runs `make lint`)
#
# Run `make` (or `make help`) to list every target.

SHELL := /bin/bash

# Backend unit-test coverage gate (CONTRACTS.md §14). Overridable: make test-backend COVERAGE_MIN=95
COVERAGE_MIN ?= 90

# Dedicated E2E/test database — separate from whatever "whmcs" database a
# developer points their own local backend at (docs/E2E.md §3), so running
# tests never clobbers local manual-testing data. Same Postgres server/port,
# different database name. Overridable: make test-backend DATABASE_URL=...
DATABASE_URL ?= postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable

# Optional Playwright filter: make test-frontend SPEC=admin-ops.spec.ts
SPEC ?=

.DEFAULT_GOAL := help
.PHONY: help up down tunnel lint guard-e2e-imports hooks test-backend test-frontend test

help: ## List available commands
	@echo "WHCMS — available make targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

## ---------------------------------------------------------------------------
## Local stack
## ---------------------------------------------------------------------------

up: ## Start the full local E2E stack (mockserver, api, seed, worker, frontend)
	bash scripts/e2e-up.sh

down: ## Stop the local E2E stack started by `make up`
	bash scripts/e2e-down.sh

tunnel: ## Start a cloudflared tunnel to the local API (:8080) and list this project's webhooks
	bash scripts/tunnel.sh

## ---------------------------------------------------------------------------
## Lint & conventions (the same checks CI runs — keep the local gate a superset)
## ---------------------------------------------------------------------------

lint: ## gofmt + golangci-lint (backend), go vet (mockserver), e2e import guard
	@echo "==> gofmt"
	@out="$$(gofmt -l backend/cmd backend/internal backend/pkg mockserver)"; \
		if [[ -n "$$out" ]]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi
	@echo "==> golangci-lint (backend)"
	$(MAKE) -C backend lint
	@echo "==> go vet (mockserver)"
	cd mockserver && go vet ./...
	@$(MAKE) guard-e2e-imports
	@echo "PASS: lint clean"

guard-e2e-imports: ## Fail if a Playwright spec bypasses tests/e2e/fixtures.ts
	@bad="$$(grep -l "from '@playwright/test'" frontend/tests/e2e/*.spec.ts 2>/dev/null || true)"; \
		if [[ -n "$$bad" ]]; then \
			echo "FAIL: specs must import { test, expect } from './fixtures', not '@playwright/test':"; \
			echo "$$bad"; \
			echo "fixtures.ts wraps page.goto to wait for SvelteKit hydration; without it,"; \
			echo "first interactions race hydration on slow machines (green locally, red in CI)."; \
			exit 1; \
		fi
	@echo "==> e2e import guard OK (all specs use ./fixtures)"

hooks: ## Install the repo git hooks (.githooks; pre-push runs `make lint`)
	git config core.hooksPath .githooks
	@echo "Installed: pre-push now runs 'make lint' (bypass once with SKIP_LINT=1 git push)."

## ---------------------------------------------------------------------------
## Tests
## ---------------------------------------------------------------------------

test-backend: ## Run backend tests + coverage, enforcing the >=90% gate (needs Postgres)
	@# The repository layer is integration-tested against real Postgres, so the
	@# >=90% coverage gate (CONTRACTS §14) runs the full Go test suite (unit +
	@# integration) over internal/... + pkg/... and needs a running DB.
	@if ! (exec 3<>/dev/tcp/127.0.0.1/5432) 2>/dev/null; then \
		echo "Postgres (:5432) is not reachable."; \
		echo "Start it (or run 'make up') — the >=$(COVERAGE_MIN)% gate includes the"; \
		echo "integration-tested repository layer, which needs a real database."; \
		exit 1; \
	fi
	@echo "==> Ensuring the E2E/test database exists ($(DATABASE_URL))"
	@bash -c 'source scripts/lib.sh && ensure_database whmcs_e2e'
	@echo "==> Ensuring DB schema is migrated"
	$(MAKE) -C backend migrate-up DATABASE_URL="$(DATABASE_URL)"
	@echo "==> Backend: build + vet + tests with coverage (unit + integration over internal/... + pkg/...)"
	$(MAKE) -C backend build vet cover-gate DATABASE_URL="$(DATABASE_URL)"
	@echo "==> Enforcing coverage gate (>= $(COVERAGE_MIN)%)"
	@cd backend && total="$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/,"",$$3); print $$3}')"; \
		if [[ -z "$$total" ]]; then echo "FAIL: could not read total coverage from coverage.out"; exit 1; fi; \
		if awk -v c="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN{exit !(c+0 >= min+0)}'; then \
			echo "PASS: total coverage $$total% meets the >= $(COVERAGE_MIN)% gate"; \
			echo "      HTML report: backend/coverage.html"; \
		else \
			echo "FAIL: total coverage $$total% is below the required $(COVERAGE_MIN)%"; \
			echo "      See uncovered lines: cd backend && go tool cover -html=coverage.out"; \
			exit 1; \
		fi

test-frontend: ## Run Playwright E2E tests (needs `make up` first). Filter with SPEC=<file>
	@$(MAKE) guard-e2e-imports
	@if ! curl -fsS -o /dev/null http://localhost:8080/healthz 2>/dev/null; then \
		echo "Backend API (:8080) is not responding — run 'make up' first."; exit 1; \
	fi
	cd frontend && npx playwright test $(SPEC)

test: lint test-backend test-frontend ## Lint, then run backend + frontend tests
