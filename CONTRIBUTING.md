# Contributing to WHCMS

Thanks for your interest in contributing! This document is the short version
of how to get a change merged. The **binding architecture contract** lives in
[`docs/CONTRACTS.md`](docs/CONTRACTS.md) — read it before touching backend
code; [`CLAUDE.md`](CLAUDE.md) is the day-to-day engineering guide (it applies
to humans too, not just AI agents).

## Getting set up

Prerequisites: Go ≥ 1.26, Node 22, PostgreSQL 18, Redis 7, and an
S3-compatible store (RustFS or MinIO) on `:9000`. Two paths:

- **Native:** follow the Quickstart in [`README.md`](README.md) — the
  `/install` wizard bootstraps the database and your first admin account.
- **Docker:** `docker compose -f deploy/docker-compose.yml --profile dev up
  -d --build` starts everything, including the mockserver.

`make help` (repo root) lists every task. `make up` / `make down` start and
stop the full local stack used by the E2E suite.

## Ground rules (enforced by CI)

1. **Backend coverage must stay above 90%.** `make test-backend` runs
   build + vet + the full unit/integration suite and fails below the gate.
   Every new exported function needs tests.
2. **Every new user-facing frontend flow ships its own Playwright E2E spec**
   in `frontend/tests/e2e/`. Preserve existing `data-testid`s, form
   `action`s, and field `name`s — the suite keys on them.
3. **Money is IDR-only, `int64` whole rupiah.** Never floats for money.
4. **Secrets never touch source control or plaintext DB columns.** Dev
   defaults for the local stack are fine; anything real comes from env or the
   admin-configurable encrypted settings.
5. **The HTTP envelope and error codes are fixed** — see
   `docs/CONTRACTS.md` §2. Don't invent new response shapes.
6. **Migrations are forward-only.** Never edit an applied migration; add a
   new pair.
7. Admin UI uses the HostPanel `hp-*` theme; client/public UI uses the
   Twenty-One `.ca` theme, with strings through the `t()` i18n helper.

## Making a change

Backend work is a vertical slice inside `backend/internal/modules/<mod>/`
(domain → migration → ports → repo → service → dto → handler → wiring →
tests) — the step-by-step recipe is in `CLAUDE.md` §3. Frontend pages pair
`+page.server.ts` (load + actions via the BFF `apiFetch`) with
`+page.svelte`.

Before opening a PR:

```bash
make test-backend            # >90% coverage gate (needs local Postgres)
cd frontend && npm run check # svelte-check, must be 0 errors
cd frontend && npm run build
make up && make test-frontend  # Playwright E2E, or SPEC=<file> for one spec
```

## Commit messages — Conventional Commits (required)

Every commit follows [Conventional Commits](https://www.conventionalcommits.org):

```
type(scope): imperative subject line

Optional body: what and why, wrapped at ~72 columns.
```

`type` ∈ `feat fix docs style refactor perf test build ci chore revert`;
`scope` is usually a backend module (`auth`, `billing`, `orders`, …) or
`frontend`, `admin`, `docs`, `deploy`, `mockserver`. Breaking changes get a
`!` and a `BREAKING CHANGE:` footer. One logical change per commit.

## Pull requests

- Keep PRs focused; describe **what** changed and **why**.
- Link related issues; include screenshots for UI changes.
- CI must be green (backend gate, svelte-check, build).
- New behavior needs to be exercised end-to-end, not just unit-tested.

## Reporting bugs & security issues

- Bugs / feature requests: open a GitHub issue using the templates.
- **Security vulnerabilities: never open a public issue** — follow
  [SECURITY.md](SECURITY.md).

## Code of Conduct

By participating you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).
