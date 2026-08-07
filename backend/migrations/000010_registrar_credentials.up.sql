-- Admin-configurable, encrypted-at-rest registrar credentials (dynamic —
-- effective without restarting the API/worker processes; env vars remain
-- the fallback for any field left blank). See docs/CONTRACTS.md §5, §10.
ALTER TABLE registrars
    ADD COLUMN reseller_id TEXT NOT NULL DEFAULT '', -- plain: an identifier, not a secret
    ADD COLUMN api_key_enc TEXT NOT NULL DEFAULT ''; -- AES-256-GCM ciphertext
