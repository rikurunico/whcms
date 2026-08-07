# WHCMS — Full-Stack E2E Guide

> How the full local stack (Postgres + Redis + RustFS + mockserver + api + worker + frontend)
> is stood up as one live system and driven with Playwright, covering PRD §13.2's 8 critical
> flows plus the per-feature specs that have grown around them.

## 1. Local topology (all on localhost, native processes — no Docker needed)

| Process | Port | Command (from repo root) |
|---|---|---|
| PostgreSQL | 5432 | already running (root/postgres). `scripts/e2e-up.sh` creates+uses its own `whmcs_e2e` database, separate from whatever `whmcs` database you use for manual local testing — see §3. |
| Redis | 6379 | already running, no password |
| RustFS (S3) | 9000 | already running (rustfsadmin/rustfsadmin, bucket `whmcs`) |
| mockserver | 9090 | `cd mockserver && go run .` |
| backend api | 8080 | `cd backend && go run ./cmd/api` |
| backend worker | (none) | `cd backend && go run ./cmd/worker` |
| frontend | 5173 | `cd frontend && npm run dev` (Playwright's own webServer can also manage this) |

## 2. Required env for api + worker during E2E (must match mockserver's defaults exactly)

```
APP_ENV=development
APP_PORT=8080
APP_BASE_URL=http://localhost:8080
FRONTEND_URL=http://localhost:5173
JWT_SECRET=<any 32+ char string>
APP_ENCRYPTION_KEY=<base64 of exactly 32 random bytes>
DATABASE_URL=postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable
REDIS_ADDR=localhost:6379
RUSTFS_ENDPOINT=http://localhost:9000
RUSTFS_ACCESS_KEY=rustfsadmin
RUSTFS_SECRET_KEY=rustfsadmin
RUSTFS_BUCKET=whmcs
RUSTFS_USE_SSL=false
DUITKU_MERCHANT_CODE=DEMO
DUITKU_API_KEY=secretkey
DUITKU_ENV=sandbox
DUITKU_BASE_URL=http://localhost:9090
RDASH_RESELLER_ID=e2e-reseller
RDASH_API_KEY=e2e-key
RDASH_BASE_URL=http://localhost:9090/v1
MAIL_DRIVER=http
MAIL_HTTP_URL=http://localhost:9090/mail/send
WORKER_CONCURRENCY=5
ADMIN_ALERT_EMAIL=admin-alerts@example.test
```

`DUITKU_MERCHANT_CODE`/`DUITKU_API_KEY` MUST equal mockserver's own `DUITKU_MERCHANT_CODE`/`DUITKU_API_KEY` env
(both default to `DEMO`/`secretkey` — leave both unset and they match automatically) or every Duitku signature
check fails with 400.

## 3. Database & storage reset

E2E/test runs use their own **`whmcs_e2e`** database — separate from the `whmcs` database a developer's own
locally-running backend uses for manual testing (via the installer wizard or a hand-set `DATABASE_URL`). This
means running `make up`/`make test-backend`/`make test-frontend` never touches or clobbers your local dev data.
`scripts/e2e-up.sh` and the root `Makefile`'s `test-backend` target both auto-create `whmcs_e2e` if it doesn't
exist yet (`scripts/lib.sh`'s `ensure_database`) — no manual step needed for the common case.

**The database is isolated, but Redis is NOT** — and asynq hands each queued job to whichever worker is
listening, regardless of which database enqueued it. So a developer's own `go run ./cmd/worker` (pointed at
`whmcs`) will happily consume jobs the E2E stack's api enqueued against `whmcs_e2e`, run them against the wrong
database, and fail them. The E2E test still waiting on that job just times out. The symptom is a slow, widely
failing suite ("waiting for mail…", services stuck `pending`) that looks nothing like its cause. Worse, `go run`
runs the real worker as a *child* process, so killing the `go run` PID leaks the child — it survives with
`PPID=1` and keeps stealing jobs invisibly. `scripts/e2e-down.sh` (which `e2e-up.sh` always runs first) now
sweeps both the wrapper and any already-orphaned child, so `make up`/`make down` clean this up automatically.
To check by hand: `pgrep -fl 'cmd/worker|go-build.*/worker$'` should be empty, and
`redis-cli --scan --pattern 'asynq:servers:*'` should list exactly one worker while the stack is up.

To force a full clean slate (wipe accumulated per-module-test leftovers), drop and recreate it explicitly:
```sh
psql -U root -h localhost -d postgres -c "DROP DATABASE IF EXISTS whmcs_e2e WITH (FORCE);" -c "CREATE DATABASE whmcs_e2e;"
```
The api binary runs migrations on startup (idempotent), so just starting `cmd/api` (pointed at `whmcs_e2e`) after
the drop/recreate re-applies schema + seed (settings, email templates, ticket departments, rdash registrar row).
RustFS: clear and recreate the `whmcs` bucket via the S3 API (same pattern used at project start — `mc` isn't
installed, use `curl --aws-sigv4` against `localhost:9000` with `rustfsadmin`/`rustfsadmin`, or a tiny Go/Python
script using the same HMAC signing) so no stray ticket-attachment objects survive from earlier module test runs.

To reset your separate **local** `whmcs` database (e.g. to start fresh with the installation wizard), run the
same drop/recreate against `whmcs` instead, then restart your local `cmd/api` — see root `README.md`.

## 4. Seed data (build `backend/cmd/seed`, per repo layout — a small idempotent CLI)

Minimum fixtures E2E flows need (use `ON CONFLICT DO NOTHING`/upsert so the seeder is safe to re-run):
- Admin user (email `admin@e2e.test`, password known to test authors, role `admin`).
- Staff user (email `staff@e2e.test`, role `staff`, permissions WITHOUT `settings` — needed for the RBAC flow
  that proves staff can't reach `/admin/settings`).
- A server row pointing at mockserver for **both** modules: hostname `localhost`, port `9090`, `use_ssl=false`,
  module `cpanel` (username/token any non-empty string, NOT `badtoken`) and a second row module `directadmin`
  (username/password any non-empty string, NOT `bad`). A server group per module referencing its server.
- Two products: one `shared_hosting` bound to the cpanel server group, `auto_setup=on_payment`, with pricing for
  at least `monthly` (a small IDR amount, e.g. 50000) and a `setup_fee` of 0; one `domain` type product (or rely
  on the domain order flow's own product-less domain-register path — check how `orders` module models a
  standalone domain purchase per its DTO on disk and seed whatever it needs).
- The `rdash` registrar row already exists from the core seed migration — update its `config` JSONB if the
  domains/orders module expects anything beyond what's there (check `domains.RegistrarRepo`/`orders` service on
  disk).
- One active coupon for a coupon-application spot-check (optional, nice-to-have, not one of the 8 mandated flows).
- Do NOT hand-craft `services`/`domains`/`invoices` rows for the "happy path" flows — those are created by
  driving the real order→pay flow through the API/UI, which is the point of E2E. You MAY need to fabricate
  state directly (DB write or an admin API call) for the **renewal/unsuspend** flow, since waiting real calendar
  days isn't feasible — see §5.5 below.

## 5. The 8 flows (PRD §13.2) — one Playwright spec file per flow, `frontend/tests/e2e/<flow>.spec.ts`

Use `data-testid` selectors documented by the frontend builders (see `docs/RECONCILE.md`'s note on the
`<span class="contents" data-testid=...>` wrapper pattern for buttons and `#field-<name>` for FormField inputs,
plus each page's own testids — grep the relevant route's `.svelte` files for `data-testid` if unsure). Use
UNIQUE test data per spec (timestamp/random-suffixed emails, domains, coupon codes) — specs may run concurrently
against the SAME live backend, so never assert exact global counts (e.g. "exactly N products in the list");
assert your own created row's presence/state instead.

1. **Register → verify → login**: fill the register form with a fresh email, capture the verification link via
   the mock mail sink (`GET http://localhost:9090/mail/messages?to=<email>` — parse the token out of the emailed
   verify-email link/body), visit it (or POST the token per the frontend's actual verify-email page contract),
   then log in with the same credentials and land on the client dashboard.
2. **Order hosting → checkout → pay → active**: browse the seeded hosting product, add to cart, checkout (as a
   just-registered-and-verified user or a fresh one created inline), land on the invoice page, choose a Duitku
   payment method, follow the mock payment page (`http://localhost:9090/payment/<reference>`, click "Bayar
   Sekarang"), get redirected back, poll until the invoice shows Paid, then confirm the service reaches `active`
   (worker must actually run for provisioning to fire — make sure the worker process is up for this spec).
3. **Check domain → order → register → active**: use the domain search (append `-available` or avoid the word
   "taken" in the chosen name so mockserver reports it available), add to cart as a domain registration, checkout
   + pay same as above, confirm the domain reaches `active` in the client domains list.
4. **Client manage service & domain**: on an active service from flow 2, change its password via the client UI
   and confirm success feedback; on the active domain from flow 3, change nameservers via the client UI and
   confirm the new values are reflected (mockserver stores and returns them).
5. **Pay renewal invoice → unsuspend**: since real calendar time can't pass in a test, fabricate the "overdue,
   about-to-be-suspended" state directly — either (a) call an admin endpoint to force a service into `suspended`
   with an open unpaid renewal invoice, or (b) directly UPDATE the service's `next_due_date` to the past and let
   an admin action (or a direct DB nudge) create the renewal invoice, then suspend it — whichever is less invasive
   given the actual admin API surface on disk (check `internal/modules/provisioning` and `internal/modules/billing`
   handler routes for anything usable, e.g. admin suspend + an admin "generate renewal invoice now" affordance;
   if nothing fits, direct SQL against the `whmcs_e2e` DB in a test setup step is acceptable ONLY for this one
   flow, clearly commented as to why). Then pay that renewal invoice via the client UI same as flow 2 and confirm
   the service returns to `active` (unsuspended).
6. **Support ticket**: as a client, open a ticket (with an attachment), confirm the mock mail sink received the
   department/admin notification, reply, confirm status transitions, close it.
7. **Admin**: log in as the seeded admin, create a NEW product from the admin UI, perform a manual provisioning
   action on an existing service (e.g. suspend then unsuspend from the admin services page), view the dashboard
   (confirm KPIs render without error) and at least one report page (revenue) without error.
8. **RBAC**: log in as the seeded staff user (no `settings` permission) and confirm `/admin/settings` is
   inaccessible (redirect, 403 page, or hidden nav — whatever the actual frontend guard does); log in as a plain
   client and confirm any `/admin/*` route redirects away / is inaccessible.

## 6. Fixing real mismatches as you go

`docs/RECONCILE.md` lists every shape the frontend GUESSED against the backend contract before the backend
modules existed. Now that both sides are real, when a flow breaks because of a mismatch: prefer fixing the
**frontend** loader/action to match the REAL backend response (the backend was built against the authoritative
`docs/MODULES.md` spec). Only add something to the **backend** if the frontend need is genuine and reasonable
and truly has no existing equivalent (e.g. a missing list endpoint) — keep such additions small and additive
(new handler method + route, not a redesign), and update `docs/RECONCILE.md`'s entry once fixed so it's not
mistaken for still-open.

## 7. Definition of done

- `frontend/tests/e2e/*.spec.ts` for all 8 flows pass headless Chromium, run together (not just individually —
  cross-test interference is a real risk since they share one backend + DB).
- No flow required disabling/skipping a real product feature to pass.
- A short `scripts/e2e-up.sh` / `scripts/e2e-down.sh` (or documented manual steps) exist so the stack can be
  reproduced.
