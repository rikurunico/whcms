-- FR-CLI-005: per-area permissions on client sub-accounts (client_contacts).
-- Client-area subset per docs/RECONCILE.md: invoices/services/domains/tickets.

ALTER TABLE client_contacts ADD COLUMN permissions JSONB NOT NULL DEFAULT '{}'::jsonb;
