# Changelog

All notable changes to WHCMS are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow
[Semantic Versioning](https://semver.org/) once tagged releases begin.

## [Unreleased]

### Added
- Modular payment-gateway registry: Duitku config (incl. base URL) editable
  live from the admin, plus a manual bank-transfer gateway with
  admin-confirmed settlement; VA/QRIS instructions render inline and pending
  payments resume on reload instead of starting over.
- Global `_layout` email wrapper, redesigned default templates, and a
  synchronous admin "Send Test Email" (migration `000020`).
- Per-user email delivery history for clients (`/account/email-history`,
  migration `000021`).
- Billing: "Invoice Selected Items" and admin-initiated prorated upgrade
  invoices.
- Admin "Pending Module Actions" queue (view/retry/dismiss stuck provisioning
  and domain jobs) with a dashboard KPI card.
- Catalog: product duplication, per-product shell/CGI access and panel
  feature-set selection; provisioning reuses control-panel packages whose
  dynamic specs already match, and supports WHM reseller package-name
  prefixes.
- Orders: `on_order` products activate immediately without payment; checkout
  invoices with nothing due auto-settle without a gateway round-trip.
- `make tunnel` (cloudflared) to expose the local API for real gateway
  callbacks.
- Broader audit-log coverage across client and admin actions.
- Open-source scaffolding: MIT LICENSE, CONTRIBUTING, SECURITY policy,
  Code of Conduct, GitHub Actions CI (backend coverage gate, lint,
  frontend check/build, Playwright E2E), issue/PR templates, dependabot.
- golangci-lint configuration (`backend/.golangci.yml`) and `make lint`.
- Prettier configuration for the frontend (`npm run format`).
- `PANEL_TLS_INSECURE_SKIP_VERIFY` env var: cPanel/WHM and DirectAdmin TLS
  certificates are now **verified by default**, with an explicit opt-out for
  self-signed panels.
- Performance indexes migration (`000014`): invoice_items related lookup,
  admin list sorts (orders/domains/tickets), housekeeping prunes, domain
  renewal cron.
- Password reset/change now revokes every outstanding session
  (`RevokeAllForUser`).
- Baseline security headers on all BFF responses (X-Frame-Options, nosniff,
  Referrer-Policy, Permissions-Policy, frame-ancestors CSP, HSTS on HTTPS).
- Production compose: required `POSTGRES_PASSWORD`, optional Redis auth,
  log rotation, `stop_grace_period`, pinned RustFS version,
  `BODY_SIZE_LIMIT` for uploads through the BFF.

### Changed
- Repository structure: `PRD.md`, `DESIGN.md`, and `DESIGN-FRONT.md` moved
  from the repo root into `docs/`.
- SMTP mailer hardened for real-world providers (implicit-TLS port 465,
  STARTTLS/auth negotiation, HELO hostname).
- Client profile: address fields persist correctly and province is required
  (registrar contact requirements).
- RDash: an existing customer is reused by email instead of erroring on
  duplicate registration.
- Production startup refuses publicly-known dev `JWT_SECRET` /
  `APP_ENCRYPTION_KEY` values.
- Several SvelteKit `load` functions now fetch independent data concurrently.
- `scripts/e2e-up.sh` generates per-checkout secrets, preflights required
  services, and installs frontend dependencies when missing.

### Fixed
- Provisioning: account creation is idempotent; username collisions and
  reserved-username rejections retry with a fresh candidate; DirectAdmin
  account creation sends the server IP and tolerates DA's no-error-key
  package dumps.
- E2E infrastructure can no longer silently target a real database, and
  teardown sweeps orphaned dev workers.
- `.env` parsing strips trailing comments.

### Removed
- Committed `backend/seed` build artifact (13.7 MB) — untracked and ignored.
