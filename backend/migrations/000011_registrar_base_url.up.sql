-- Admin-configurable registrar API base URL ("custom endpoint") — lets an
-- operator point the RDash adapter at a different endpoint (e.g. a local
-- mockserver vs the real Dewabiz API) without restarting the process. Blank
-- falls back to RDASH_BASE_URL (see docs/CONTRACTS.md §5, §10).
ALTER TABLE registrars
    ADD COLUMN base_url TEXT NOT NULL DEFAULT '';
