#!/usr/bin/env bash
# Starts the full WHCMS local E2E stack in the background:
#   mockserver (9090) -> backend api (8080, runs migrations) -> seed data ->
#   backend worker -> frontend (5173).
#
# Always tears down first (scripts/e2e-down.sh) before spawning its own
# dedicated stack: a process already listening on these ports could be a
# developer's manually-started api/worker pointed at their real dev database
# (backend/.env) rather than this stack's isolated whmcs_e2e, so it must not
# be reused silently.
#
# PIDs are written to repo_root/.e2e-logs/*.pid; logs to
# repo_root/.e2e-logs/*.log.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$REPO_ROOT/.e2e-logs"
mkdir -p "$LOG_DIR"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

# Kill whatever is on 9090/8080/5173 (and stray worker-bins) unconditionally
# rather than guessing whose processes they are.
echo "[e2e-up] tearing down any existing stack first for a clean, isolated run"
"$SCRIPT_DIR/e2e-down.sh"

# --- Preflight: the stack needs local PG/Redis/S3 already running -----------
preflight() {
	local port="$1" name="$2"
	if ! (exec 3<>"/dev/tcp/127.0.0.1/${port}") 2>/dev/null; then
		echo "[e2e-up] ERROR: ${name} (:${port}) is not reachable." >&2
		echo "[e2e-up] Start the prerequisite services first — see README.md (PostgreSQL 18, Redis, RustFS/MinIO)." >&2
		exit 1
	fi
	exec 3>&- 2>/dev/null || true
}
preflight 5432 "PostgreSQL"
preflight 6379 "Redis"
preflight 9000 "RustFS/S3"

# --- Required env (docs/E2E.md §2) -----------------------------------------
export APP_ENV=development
export APP_PORT=8080
export APP_BASE_URL=http://localhost:8080
export FRONTEND_URL=http://localhost:5173
# Per-checkout generated secrets (no committed literals). Persisted in
# $LOG_DIR so partial restarts (e.g. only the worker died) keep matching the
# already-running API. If you delete this file, also drop the whmcs_e2e
# database: settings encrypted under the old key become undecryptable.
SECRETS_FILE="$LOG_DIR/e2e-secrets.env"
if [[ -z "${JWT_SECRET:-}" || -z "${APP_ENCRYPTION_KEY:-}" ]]; then
	if [[ ! -f "$SECRETS_FILE" ]]; then
		{
			echo "JWT_SECRET=$(openssl rand -base64 32)"
			echo "APP_ENCRYPTION_KEY=$(openssl rand -base64 32)"
		} >"$SECRETS_FILE"
	fi
	# shellcheck source=/dev/null
	source "$SECRETS_FILE"
fi
export JWT_SECRET APP_ENCRYPTION_KEY
export DATABASE_URL="${DATABASE_URL:-postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable}"
export REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
export RUSTFS_ENDPOINT="${RUSTFS_ENDPOINT:-http://localhost:9000}"
export RUSTFS_ACCESS_KEY="${RUSTFS_ACCESS_KEY:-rustfsadmin}"
export RUSTFS_SECRET_KEY="${RUSTFS_SECRET_KEY:-rustfsadmin}"
export RUSTFS_BUCKET="${RUSTFS_BUCKET:-whmcs}"
export RUSTFS_USE_SSL=false
export DUITKU_MERCHANT_CODE="${DUITKU_MERCHANT_CODE:-DEMO}"
export DUITKU_API_KEY="${DUITKU_API_KEY:-secretkey}"
export DUITKU_ENV=sandbox
export DUITKU_BASE_URL=http://localhost:9090
export RDASH_RESELLER_ID="${RDASH_RESELLER_ID:-e2e-reseller}"
export RDASH_API_KEY="${RDASH_API_KEY:-e2e-key}"
export RDASH_BASE_URL=http://localhost:9090/v1
# Cloudflare Turnstile CAPTCHA (optional; off by default via settings). Dummy
# "always passes" test secret + mockserver siteverify so enabling it in dev
# works offline and E2E stays deterministic.
export TURNSTILE_SECRET_KEY="${TURNSTILE_SECRET_KEY:-1x0000000000000000000000000000000AA}"
export TURNSTILE_VERIFY_URL="${TURNSTILE_VERIFY_URL:-http://localhost:9090/turnstile/v0/siteverify}"
export MAIL_DRIVER=http
export MAIL_HTTP_URL=http://localhost:9090/mail/send
export WORKER_CONCURRENCY=10
export ADMIN_ALERT_EMAIL=admin-alerts@example.test

wait_for() {
	local url="$1" name="$2" tries="${3:-60}"
	for ((i = 0; i < tries; i++)); do
		if curl -fsS -o /dev/null "$url" 2>/dev/null; then
			echo "[e2e-up] $name is up ($url)"
			return 0
		fi
		sleep 1
	done
	echo "[e2e-up] ERROR: $name did not become healthy at $url within ${tries}s" >&2
	echo "[e2e-up] --- last 40 lines of its log ---" >&2
	tail -n 40 "$LOG_DIR/${name}.log" >&2 || true
	exit 1
}

port_free() {
	! lsof -i ":$1" -sTCP:LISTEN >/dev/null 2>&1
}

# --- 1. mockserver ----------------------------------------------------------
if port_free 9090; then
	echo "[e2e-up] starting mockserver on :9090"
	(cd "$REPO_ROOT/mockserver" && MOCK_PORT=9090 nohup go run . >"$LOG_DIR/mockserver.log" 2>&1 & echo $! >"$LOG_DIR/mockserver.pid")
else
	echo "[e2e-up] :9090 already listening, assuming mockserver is up"
fi
wait_for "http://localhost:9090/healthz" mockserver

# --- 2. backend api (runs migrations on start) ------------------------------
# This DB is dedicated to E2E/test runs — separate from whatever "whmcs"
# database a developer uses for manual local testing (docs/E2E.md §3).
ensure_database whmcs_e2e
if port_free 8080; then
	echo "[e2e-up] starting backend api on :8080"
	(cd "$REPO_ROOT/backend" && nohup go run ./cmd/api >"$LOG_DIR/api.log" 2>&1 & echo $! >"$LOG_DIR/api.pid")
else
	echo "[e2e-up] :8080 already listening, assuming api is up"
fi
wait_for "http://localhost:8080/healthz" api 90
wait_for "http://localhost:8080/readyz" api-ready 30

# --- 3. seed data ------------------------------------------------------------
echo "[e2e-up] running seeder"
(cd "$REPO_ROOT/backend" && go run ./cmd/seed) 2>&1 | tee "$LOG_DIR/seed.log"

# --- 4. backend worker -------------------------------------------------------
if [[ ! -f "$LOG_DIR/worker.pid" ]] || ! kill -0 "$(cat "$LOG_DIR/worker.pid")" 2>/dev/null; then
	# Built to a real binary and run directly, NOT `go run ./cmd/worker`:
	# `go run` execs the compiled binary as a child process, so the pidfile
	# would hold the wrapper's PID and e2e-down.sh could never reliably kill
	# the worker itself — and the worker binds no port, so the port-based
	# teardown fallback can't catch the leak either.
	echo "[e2e-up] building backend worker"
	(cd "$REPO_ROOT/backend" && go build -o "$LOG_DIR/worker-bin" ./cmd/worker)
	echo "[e2e-up] starting backend worker"
	# Backgrounded directly (no `cd &&`, no `( ... & )` subshell): a compound
	# command backgrounds via an intermediate shell, and $! would capture that
	# wrapper's PID instead of worker-bin's.
	nohup "$LOG_DIR/worker-bin" >"$LOG_DIR/worker.log" 2>&1 &
	echo $! >"$LOG_DIR/worker.pid"
	sleep 3
	if ! grep -q "worker running" "$LOG_DIR/worker.log"; then
		echo "[e2e-up] ERROR: worker did not log 'worker running'" >&2
		tail -n 40 "$LOG_DIR/worker.log" >&2
		exit 1
	fi
	echo "[e2e-up] worker is up ($(grep "worker running" "$LOG_DIR/worker.log" | tail -1))"
else
	echo "[e2e-up] worker already running (pid $(cat "$LOG_DIR/worker.pid"))"
fi

# --- 5. frontend --------------------------------------------------------------
if port_free 5173; then
	if [[ ! -d "$REPO_ROOT/frontend/node_modules" ]]; then
		echo "[e2e-up] frontend/node_modules missing — running npm ci"
		(cd "$REPO_ROOT/frontend" && npm ci)
	fi
	echo "[e2e-up] starting frontend on :5173"
	(cd "$REPO_ROOT/frontend" && API_URL=http://localhost:8080 nohup npm run dev >"$LOG_DIR/frontend.log" 2>&1 & echo $! >"$LOG_DIR/frontend.pid")
else
	echo "[e2e-up] :5173 already listening, assuming frontend is up"
fi
wait_for "http://localhost:5173/" frontend 60

echo "[e2e-up] stack is up:"
echo "  mockserver : http://localhost:9090  (pid $(cat "$LOG_DIR/mockserver.pid" 2>/dev/null || echo '?'))"
echo "  api        : http://localhost:8080  (pid $(cat "$LOG_DIR/api.pid" 2>/dev/null || echo '?'))"
echo "  worker     : (no port)              (pid $(cat "$LOG_DIR/worker.pid" 2>/dev/null || echo '?'))"
echo "  frontend   : http://localhost:5173  (pid $(cat "$LOG_DIR/frontend.pid" 2>/dev/null || echo '?'))"
echo "  logs       : $LOG_DIR/*.log"
