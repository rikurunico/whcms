-- Performance indexes for hot paths identified in the 2026-07 audit.

-- Order detail + billing renewal/dunning crons resolve invoice items by their
-- source entity (related_type IN order_item/service_renewal/domain_renewal…).
CREATE INDEX IF NOT EXISTS invoice_items_related_idx
    ON invoice_items (related_type, related_id);

-- Admin list pagination sorts (orders/domains: ORDER BY created_at DESC, id
-- DESC; tickets: ORDER BY COALESCE(last_reply_at, created_at) DESC, id DESC).
CREATE INDEX IF NOT EXISTS orders_created_at_idx
    ON orders (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS domains_created_at_idx
    ON domains (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS tickets_last_activity_idx
    ON tickets ((COALESCE(last_reply_at, created_at)) DESC, id DESC);

-- Housekeeping prune jobs delete by age.
CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx ON audit_logs (created_at);
CREATE INDEX IF NOT EXISTS email_log_created_at_idx ON email_log (created_at);

-- Domain renewal cron (analogous to services_next_due_date_active_idx).
CREATE INDEX IF NOT EXISTS domains_next_due_date_active_idx
    ON domains (next_due_date)
    WHERE status = 'active' AND auto_renew;
