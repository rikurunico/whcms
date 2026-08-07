-- Module-phase contract additions: recurring coupons on services, pending
-- upgrades, user locale preference, service_upgrade invoice item type.

ALTER TABLE services ADD COLUMN coupon_id BIGINT NULL REFERENCES coupons(id);
ALTER TABLE services ADD COLUMN pending_upgrade JSONB NULL;
CREATE INDEX services_coupon_id_idx ON services (coupon_id);

ALTER TABLE users ADD COLUMN locale TEXT NOT NULL DEFAULT 'id';

ALTER TABLE invoice_items DROP CONSTRAINT IF EXISTS invoice_items_related_type_check;
ALTER TABLE invoice_items ADD CONSTRAINT invoice_items_related_type_check
    CHECK (related_type IN ('order_item','service_renewal','domain_renewal','service_upgrade','late_fee','deposit','manual'));
