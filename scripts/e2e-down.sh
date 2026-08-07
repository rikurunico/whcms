#!/usr/bin/env bash
# Stops the WHCMS local E2E stack started by scripts/e2e-up.sh, killing
# each process by the PID file it wrote to .e2e-logs/*.pid.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$REPO_ROOT/.e2e-logs"

for name in frontend worker api mockserver; do
	pidfile="$LOG_DIR/${name}.pid"
	if [[ -f "$pidfile" ]]; then
		pid="$(cat "$pidfile")"
		if kill -0 "$pid" 2>/dev/null; then
			echo "[e2e-down] stopping $name (pid $pid)"
			kill "$pid" 2>/dev/null
			for _ in $(seq 1 10); do
				kill -0 "$pid" 2>/dev/null || break
				sleep 0.5
			done
			kill -9 "$pid" 2>/dev/null || true
		else
			echo "[e2e-down] $name (pid $pid) not running"
		fi
		rm -f "$pidfile"
	else
		echo "[e2e-down] no pidfile for $name, skipping"
	fi
done

# go run spawns a child binary; make sure nothing is left listening.
for port in 9090 8080 5173; do
	pid="$(lsof -ti ":$port" -sTCP:LISTEN 2>/dev/null || true)"
	if [[ -n "$pid" ]]; then
		echo "[e2e-down] port $port still held by pid $pid, killing"
		kill -9 "$pid" 2>/dev/null || true
	fi
done

# The worker binds no port, so the port-based fallback above can't catch it —
# sweep for any stray compiled worker binary regardless of how the
# pidfile-based kill went.
stray="$(pgrep -f "$LOG_DIR/worker-bin" 2>/dev/null || true)"
if [[ -n "$stray" ]]; then
	echo "[e2e-down] killing stray worker binary: $stray"
	kill -9 $stray 2>/dev/null || true
fi

# Also sweep a developer's own `go run ./cmd/worker`: asynq hands each queued
# job to whichever worker listens on the shared Redis, so a dev worker on the
# `whmcs` database consumes jobs the E2E stack enqueued against `whmcs_e2e`,
# and the spec waiting on that mail/provision job times out with no obvious
# cause. Kill unconditionally, and walk the process tree (go run's child
# binary path is not stable across toolchains, so a path pattern alone can
# miss it).
wrappers="$(pgrep -f 'go run [^|]*cmd/worker' 2>/dev/null || true)"
for wrapper in $wrappers; do
	kids="$(pgrep -P "$wrapper" 2>/dev/null || true)"
	if [[ -n "$kids" ]]; then
		echo "[e2e-down] killing non-stack worker child of $wrapper (would steal E2E jobs via shared Redis): $kids"
		kill -9 $kids 2>/dev/null || true
	fi
	echo "[e2e-down] killing non-stack 'go run ./cmd/worker' wrapper: $wrapper"
	kill -9 "$wrapper" 2>/dev/null || true
done

# Backstop for children already orphaned by an earlier wrapper-only kill
# (parent gone, so the tree walk can't reach them). Matches a Go build
# artifact named exactly `worker`; this stack's own binary is
# `.e2e-logs/worker-bin` and never matches.
orphans="$(pgrep -f 'go-build[^ ]*/worker$' 2>/dev/null || true)"
if [[ -n "$orphans" ]]; then
	echo "[e2e-down] killing orphaned worker binary (would steal E2E jobs via shared Redis): $orphans"
	kill -9 $orphans 2>/dev/null || true
fi

echo "[e2e-down] done"
