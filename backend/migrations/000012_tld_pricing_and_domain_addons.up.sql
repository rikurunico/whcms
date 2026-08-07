-- Admin-configurable domain selling: per-TLD pricing (full 1-10 year
-- register/renew matrix + flat transfer price + min/max registrable years),
-- manually-curated premium domain pricing overrides, and a fixed catalog of
-- domain addons (ID Protection, DNS Management, Email Forwarding) that
-- customers can attach to a domain. See docs/CONTRACTS.md and the domains
-- module for how these feed order pricing.

CREATE TABLE tld_pricing (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tld             TEXT        NOT NULL UNIQUE, -- no leading dot, e.g. 'com', 'co.id'
    registrar_id    BIGINT      NOT NULL REFERENCES registrars(id),
    active          BOOLEAN     NOT NULL DEFAULT TRUE,
    min_years       INTEGER     NOT NULL DEFAULT 1,
    max_years       INTEGER     NOT NULL DEFAULT 10,
    register_prices JSONB       NOT NULL DEFAULT '{}'::jsonb, -- {"1": 150000, "2": 290000, ...} whole IDR
    renew_prices    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    transfer_price  BIGINT      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (min_years >= 1 AND max_years <= 10 AND min_years <= max_years)
);
CREATE TRIGGER tld_pricing_updated_at BEFORE UPDATE ON tld_pricing
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE premium_domain_pricing (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain_name    TEXT        NOT NULL UNIQUE, -- exact FQDN, lowercase
    register_price BIGINT      NOT NULL DEFAULT 0,
    renew_price    BIGINT      NOT NULL DEFAULT 0,
    transfer_price BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER premium_domain_pricing_updated_at BEFORE UPDATE ON premium_domain_pricing
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE domain_addons (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT        NOT NULL UNIQUE CHECK (key IN ('id_protection', 'dns_management', 'email_forwarding')),
    name       TEXT        NOT NULL,
    price      BIGINT      NOT NULL DEFAULT 0, -- annual, whole IDR
    active     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER domain_addons_updated_at BEFORE UPDATE ON domain_addons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO domain_addons (key, name, price, active) VALUES
    ('id_protection', 'ID Protection', 0, FALSE),
    ('dns_management', 'DNS Management', 0, FALSE),
    ('email_forwarding', 'Email Forwarding', 0, FALSE);

-- DNS management addon gates GET/PUT /domains/:id/dns going forward; existing
-- domains are grandfathered (kept enabled) so today's free access is not
-- retroactively revoked by this migration.
ALTER TABLE domains
    ADD COLUMN dns_management_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN email_forwarding_enabled BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE domains SET dns_management_enabled = TRUE;
