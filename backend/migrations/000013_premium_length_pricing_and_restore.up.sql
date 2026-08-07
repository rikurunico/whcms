-- Restore/redemption fee per TLD (mirrors the RDash/Dewabiz `redemption`
-- field on GET /account/prices — a flat per-extension fee, same granularity
-- as transfer_price). Captured for display/reference only for now; the
-- functional restore workflow (grace-period detection, RegistrarModule
-- restore call) is a separate follow-up.
ALTER TABLE tld_pricing
    ADD COLUMN restore_price BIGINT NOT NULL DEFAULT 0;

-- Premium pricing by TLD + registrable-label character length (e.g. Dewabiz's
-- "Limited Character" table: .id 2-char/3-char/4-char each carry a distinct
-- price). Takes precedence over standard TLD pricing but below an exact
-- premium_domain_pricing override. One flat price covers register/renew/
-- transfer for that tier (registrars don't break these out further).
CREATE TABLE premium_length_pricing (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tld         TEXT        NOT NULL, -- no leading dot, e.g. "id", "co.id"
    char_length INTEGER     NOT NULL CHECK (char_length BETWEEN 1 AND 63),
    price       BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tld, char_length)
);
CREATE TRIGGER premium_length_pricing_updated_at BEFORE UPDATE ON premium_length_pricing
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
