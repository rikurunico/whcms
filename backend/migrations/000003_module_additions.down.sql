ALTER TABLE invoice_items DROP CONSTRAINT IF EXISTS invoice_items_related_type_check;
ALTER TABLE invoice_items ADD CONSTRAINT invoice_items_related_type_check
    CHECK (related_type IN ('order_item','service_renewal','domain_renewal','late_fee','deposit','manual'));

ALTER TABLE users DROP COLUMN locale;

DROP INDEX IF EXISTS services_coupon_id_idx;
ALTER TABLE services DROP COLUMN pending_upgrade;
ALTER TABLE services DROP COLUMN coupon_id;
