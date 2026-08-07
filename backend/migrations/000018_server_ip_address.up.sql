-- Real DirectAdmin's CMD_API_ACCOUNT_USER requires a valid IP from the
-- reseller's own pool on every account create — unlike WHM's createacct,
-- which auto-assigns a shared IP when the ip param is omitted. Optional,
-- blank by default (preserves current behavior for cPanel/WHM servers,
-- which don't need one).
ALTER TABLE servers ADD COLUMN ip_address TEXT NOT NULL DEFAULT '';
