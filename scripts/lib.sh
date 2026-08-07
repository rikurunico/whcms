# Shared helpers sourced by scripts/e2e-up.sh and the root Makefile's
# test-backend target. Not executable on its own.

# ensure_database creates the named Postgres database on localhost:5432
# (root/postgres) if it doesn't already exist. Idempotent; never touches an
# existing database, never drops/recreates.
ensure_database() {
	local dbname="$1"
	if ! psql -U root -h localhost -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '${dbname}'" 2>/dev/null | grep -q 1; then
		echo "[ensure_database] creating database '${dbname}'"
		createdb -U root -h localhost "${dbname}"
	fi
}
