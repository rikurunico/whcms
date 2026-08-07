-- Dynamic / custom-spec products: customers configure their own hosting specs
-- (disk, bandwidth, addon domains, ...) which are priced per-unit and, after
-- payment, provisioned as a real per-service package on the control panel.
-- Enums are TEXT + CHECK constraints matching domain enum values exactly.

-- products.configurable: flags a product whose specs the customer configures.
ALTER TABLE products ADD COLUMN configurable BOOLEAN NOT NULL DEFAULT FALSE;

-- product_specs: one configurable knob on a product (e.g. disk, bandwidth). The
-- provision_key maps the knob to a canonical panel parameter (see domain.PanelParam).
CREATE TABLE product_specs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id      BIGINT      NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    key             TEXT        NOT NULL,
    label           TEXT        NOT NULL DEFAULT '',
    provision_key   TEXT        NOT NULL
                    CHECK (provision_key IN ('disk','bandwidth','addon_domains','subdomains',
                           'parked_domains','email_accounts','databases','ftp_accounts')),
    unit            TEXT        NOT NULL DEFAULT 'count'
                    CHECK (unit IN ('gb','mb','count')),
    included_qty    BIGINT      NOT NULL DEFAULT 0,
    min_qty         BIGINT      NOT NULL DEFAULT 0,
    max_qty         BIGINT      NOT NULL DEFAULT 0, -- 0 = unbounded
    step_qty        BIGINT      NOT NULL DEFAULT 1 CHECK (step_qty >= 1),
    default_qty     BIGINT      NOT NULL DEFAULT 0,
    allow_unlimited BOOLEAN     NOT NULL DEFAULT FALSE,
    sort            INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, key),
    UNIQUE (product_id, provision_key),
    CHECK (included_qty >= 0 AND min_qty >= 0),
    CHECK (max_qty = 0 OR max_qty >= min_qty)
);
CREATE INDEX product_specs_product_id_idx ON product_specs (product_id);
CREATE TRIGGER product_specs_updated_at BEFORE UPDATE ON product_specs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- product_spec_pricing: per-cycle IDR unit pricing for a spec (mirrors product_pricing).
-- unit_price is charged per spec-unit above included_qty; unlimited_price is a flat
-- add-on applied when the customer chooses "unlimited".
CREATE TABLE product_spec_pricing (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    spec_id         BIGINT      NOT NULL REFERENCES product_specs(id) ON DELETE CASCADE,
    cycle           TEXT        NOT NULL
                    CHECK (cycle IN ('one_time','monthly','quarterly','semiannually','annually','biennially')),
    unit_price      BIGINT      NOT NULL DEFAULT 0,
    unlimited_price BIGINT      NOT NULL DEFAULT 0,
    currency        TEXT        NOT NULL DEFAULT 'IDR',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (spec_id, cycle, currency),
    CHECK (unit_price >= 0 AND unlimited_price >= 0),
    CHECK (currency = 'IDR')
);
CREATE TRIGGER product_spec_pricing_updated_at BEFORE UPDATE ON product_spec_pricing
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
