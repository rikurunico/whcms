DROP TABLE IF EXISTS product_spec_pricing;
DROP TABLE IF EXISTS product_specs;
ALTER TABLE IF EXISTS products DROP COLUMN IF EXISTS configurable;
