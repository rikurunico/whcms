-- WHCMS initial schema (CONTRACTS.md §5).
-- Enums are TEXT + CHECK constraints matching §4 exactly.
-- Money is BIGINT whole IDR. IDs are BIGINT GENERATED ALWAYS AS IDENTITY.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- updated_at maintenance trigger --------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- users -----------------------------------------------------------------------
CREATE TABLE users (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email             TEXT        NOT NULL UNIQUE,
    password_hash     TEXT        NOT NULL,
    role              TEXT        NOT NULL DEFAULT 'client'
                      CHECK (role IN ('admin','staff','client')),
    status            TEXT        NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active','inactive')),
    permissions       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    twofa_secret_enc  TEXT        NOT NULL DEFAULT '',
    twofa_enabled     BOOLEAN     NOT NULL DEFAULT FALSE,
    email_verified_at TIMESTAMPTZ NULL,
    last_login_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_email_trgm_idx ON users USING gin (email gin_trgm_ops);
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- clients ---------------------------------------------------------------------
CREATE TABLE clients (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        BIGINT      NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    first_name     TEXT        NOT NULL DEFAULT '',
    last_name      TEXT        NOT NULL DEFAULT '',
    company        TEXT        NOT NULL DEFAULT '',
    address1       TEXT        NOT NULL DEFAULT '',
    address2       TEXT        NOT NULL DEFAULT '',
    city           TEXT        NOT NULL DEFAULT '',
    state          TEXT        NOT NULL DEFAULT '',
    postcode       TEXT        NOT NULL DEFAULT '',
    country        TEXT        NOT NULL DEFAULT 'ID',
    phone          TEXT        NOT NULL DEFAULT '',
    currency       TEXT        NOT NULL DEFAULT 'IDR',
    credit_balance BIGINT      NOT NULL DEFAULT 0 CHECK (credit_balance >= 0),
    status         TEXT        NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active','inactive','closed')),
    notes_admin    TEXT        NOT NULL DEFAULT '',
    -- helper column for trigram/ILIKE search (name search; email search via users)
    search_name    TEXT GENERATED ALWAYS AS (first_name || ' ' || last_name || ' ' || company) STORED,
    deleted_at     TIMESTAMPTZ NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX clients_search_name_trgm_idx ON clients USING gin (search_name gin_trgm_ops);
CREATE INDEX clients_status_idx ON clients (status);
CREATE TRIGGER clients_updated_at BEFORE UPDATE ON clients
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- client_contacts ---------------------------------------------------------------
CREATE TABLE client_contacts (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id  BIGINT      NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    first_name TEXT        NOT NULL DEFAULT '',
    last_name  TEXT        NOT NULL DEFAULT '',
    email      TEXT        NOT NULL DEFAULT '',
    phone      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX client_contacts_client_id_idx ON client_contacts (client_id);
CREATE TRIGGER client_contacts_updated_at BEFORE UPDATE ON client_contacts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- product_groups ----------------------------------------------------------------
CREATE TABLE product_groups (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    sort       INTEGER     NOT NULL DEFAULT 0,
    hidden     BOOLEAN     NOT NULL DEFAULT FALSE,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER product_groups_updated_at BEFORE UPDATE ON product_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- server_groups (before products, which reference it) ---------------------------
CREATE TABLE server_groups (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    strategy   TEXT        NOT NULL DEFAULT 'round_robin'
               CHECK (strategy IN ('round_robin','least_used')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER server_groups_updated_at BEFORE UPDATE ON server_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- products ----------------------------------------------------------------------
CREATE TABLE products (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id               BIGINT      NOT NULL REFERENCES product_groups(id),
    name                   TEXT        NOT NULL,
    slug                   TEXT        NOT NULL UNIQUE,
    description            TEXT        NOT NULL DEFAULT '',
    type                   TEXT        NOT NULL
                           CHECK (type IN ('shared_hosting','reseller_hosting','domain','other')),
    module                 TEXT        NOT NULL DEFAULT 'none'
                           CHECK (module IN ('cpanel','directadmin','none')),
    server_group_id        BIGINT      NULL REFERENCES server_groups(id),
    package_name           TEXT        NOT NULL DEFAULT '',
    auto_setup             TEXT        NOT NULL DEFAULT 'on_payment'
                           CHECK (auto_setup IN ('on_payment','on_order','manual')),
    stock_enabled          BOOLEAN     NOT NULL DEFAULT FALSE,
    stock_qty              INTEGER     NOT NULL DEFAULT 0,
    hidden                 BOOLEAN     NOT NULL DEFAULT FALSE,
    sort                   INTEGER     NOT NULL DEFAULT 0,
    welcome_email_template TEXT        NOT NULL DEFAULT '',
    deleted_at             TIMESTAMPTZ NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX products_group_id_idx ON products (group_id);
CREATE INDEX products_server_group_id_idx ON products (server_group_id);
CREATE TRIGGER products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- product_pricing ----------------------------------------------------------------
CREATE TABLE product_pricing (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id BIGINT      NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    cycle      TEXT        NOT NULL
               CHECK (cycle IN ('one_time','monthly','quarterly','semiannually','annually','biennially')),
    price      BIGINT      NOT NULL DEFAULT 0,
    setup_fee  BIGINT      NOT NULL DEFAULT 0,
    currency   TEXT        NOT NULL DEFAULT 'IDR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, cycle, currency)
);
CREATE TRIGGER product_pricing_updated_at BEFORE UPDATE ON product_pricing
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- configurable options -------------------------------------------------------------
CREATE TABLE configurable_option_groups (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER configurable_option_groups_updated_at BEFORE UPDATE ON configurable_option_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE configurable_options (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id   BIGINT      NOT NULL REFERENCES configurable_option_groups(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    sort       INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX configurable_options_group_id_idx ON configurable_options (group_id);
CREATE TRIGGER configurable_options_updated_at BEFORE UPDATE ON configurable_options
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE configurable_option_values (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    option_id    BIGINT      NOT NULL REFERENCES configurable_options(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    price_deltas JSONB       NOT NULL DEFAULT '{}'::jsonb, -- per-cycle deltas {"monthly":10000}
    sort         INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX configurable_option_values_option_id_idx ON configurable_option_values (option_id);
CREATE TRIGGER configurable_option_values_updated_at BEFORE UPDATE ON configurable_option_values
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- coupons ---------------------------------------------------------------------------
CREATE TABLE coupons (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code       TEXT        NOT NULL UNIQUE,
    type       TEXT        NOT NULL CHECK (type IN ('percentage','fixed')),
    value      BIGINT      NOT NULL DEFAULT 0,
    applies_to JSONB       NOT NULL DEFAULT '{}'::jsonb,
    max_uses   INTEGER     NOT NULL DEFAULT 0, -- 0 = unlimited
    used_count INTEGER     NOT NULL DEFAULT 0,
    recurring  BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NULL,
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER coupons_updated_at BEFORE UPDATE ON coupons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- orders -----------------------------------------------------------------------------
CREATE TABLE orders (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_number TEXT        NOT NULL UNIQUE,
    client_id    BIGINT      NOT NULL REFERENCES clients(id),
    status       TEXT        NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','active','fraud','cancelled')),
    subtotal     BIGINT      NOT NULL DEFAULT 0,
    discount     BIGINT      NOT NULL DEFAULT 0,
    tax_total    BIGINT      NOT NULL DEFAULT 0,
    total        BIGINT      NOT NULL DEFAULT 0,
    coupon_id    BIGINT      NULL REFERENCES coupons(id),
    ip           TEXT        NOT NULL DEFAULT '',
    notes        TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX orders_client_id_idx ON orders (client_id);
CREATE INDEX orders_coupon_id_idx ON orders (coupon_id);
CREATE INDEX orders_status_idx ON orders (status);
CREATE TRIGGER orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- services (before order_items back-ref; server FK added after servers) -------------
CREATE TABLE services (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id         BIGINT      NOT NULL REFERENCES clients(id),
    order_item_id     BIGINT      NULL, -- FK added below after order_items exists
    product_id        BIGINT      NOT NULL REFERENCES products(id),
    server_id         BIGINT      NULL, -- FK added below after servers exists
    domain            TEXT        NOT NULL DEFAULT '',
    username          TEXT        NOT NULL DEFAULT '',
    password_enc      TEXT        NOT NULL DEFAULT '',
    status            TEXT        NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','active','suspended','terminated','cancelled')),
    billing_cycle     TEXT        NOT NULL
                      CHECK (billing_cycle IN ('one_time','monthly','quarterly','semiannually','annually','biennially')),
    recurring_amount  BIGINT      NOT NULL DEFAULT 0,
    setup_fee         BIGINT      NOT NULL DEFAULT 0,
    next_due_date     DATE        NULL,
    registration_date DATE        NULL,
    terminated_at     TIMESTAMPTZ NULL,
    suspend_reason    TEXT        NOT NULL DEFAULT '',
    panel_meta        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    notes             TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX services_client_id_idx ON services (client_id);
CREATE INDEX services_product_id_idx ON services (product_id);
CREATE INDEX services_server_id_idx ON services (server_id);
CREATE INDEX services_next_due_date_active_idx ON services (next_due_date) WHERE status = 'active';
CREATE INDEX services_status_idx ON services (status);
CREATE TRIGGER services_updated_at BEFORE UPDATE ON services
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- registrars ---------------------------------------------------------------------
CREATE TABLE registrars (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE, -- 'rdash'
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    config     JSONB       NOT NULL DEFAULT '{}'::jsonb, -- non-secret config only
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER registrars_updated_at BEFORE UPDATE ON registrars
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- domains ------------------------------------------------------------------------
CREATE TABLE domains (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id         BIGINT      NOT NULL REFERENCES clients(id),
    registrar_id      BIGINT      NOT NULL REFERENCES registrars(id),
    name              TEXT        NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','active','pending_transfer','expired','cancelled')),
    registration_date DATE        NULL,
    expiry_date       DATE        NULL,
    next_due_date     DATE        NULL,
    recurring_amount  BIGINT      NOT NULL DEFAULT 0,
    billing_cycle     TEXT        NOT NULL DEFAULT 'annually'
                      CHECK (billing_cycle IN ('one_time','monthly','quarterly','semiannually','annually','biennially')),
    auto_renew        BOOLEAN     NOT NULL DEFAULT TRUE,
    nameservers       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    epp_code_enc      TEXT        NOT NULL DEFAULT '',
    id_protection     BOOLEAN     NOT NULL DEFAULT FALSE,
    registrar_meta    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- unique among non-cancelled domains ("unique-active")
CREATE UNIQUE INDEX domains_name_active_uniq ON domains (name) WHERE status <> 'cancelled';
CREATE INDEX domains_client_id_idx ON domains (client_id);
CREATE INDEX domains_registrar_id_idx ON domains (registrar_id);
CREATE INDEX domains_expiry_date_idx ON domains (expiry_date);
CREATE INDEX domains_status_idx ON domains (status);
CREATE TRIGGER domains_updated_at BEFORE UPDATE ON domains
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- dns_records (optional cache) -----------------------------------------------------
CREATE TABLE dns_records (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain_id  BIGINT      NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    type       TEXT        NOT NULL,
    host       TEXT        NOT NULL DEFAULT '',
    value      TEXT        NOT NULL DEFAULT '',
    ttl        INTEGER     NOT NULL DEFAULT 3600,
    prio       INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX dns_records_domain_id_idx ON dns_records (domain_id);
CREATE TRIGGER dns_records_updated_at BEFORE UPDATE ON dns_records
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- order_items -----------------------------------------------------------------------
CREATE TABLE order_items (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id    BIGINT      NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    item_type   TEXT        NOT NULL
                CHECK (item_type IN ('product','domain_register','domain_transfer')),
    product_id  BIGINT      NULL REFERENCES products(id),
    description TEXT        NOT NULL DEFAULT '',
    domain      TEXT        NOT NULL DEFAULT '',
    cycle       TEXT        NOT NULL
                CHECK (cycle IN ('one_time','monthly','quarterly','semiannually','annually','biennially')),
    unit_price  BIGINT      NOT NULL DEFAULT 0,
    setup_fee   BIGINT      NOT NULL DEFAULT 0,
    options     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    service_id  BIGINT      NULL REFERENCES services(id), -- back-ref filled on activation
    domain_id   BIGINT      NULL REFERENCES domains(id),  -- back-ref filled on activation
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX order_items_order_id_idx ON order_items (order_id);
CREATE INDEX order_items_product_id_idx ON order_items (product_id);
CREATE INDEX order_items_service_id_idx ON order_items (service_id);
CREATE INDEX order_items_domain_id_idx ON order_items (domain_id);
CREATE TRIGGER order_items_updated_at BEFORE UPDATE ON order_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- now services.order_item_id FK is resolvable
ALTER TABLE services
    ADD CONSTRAINT services_order_item_id_fkey
    FOREIGN KEY (order_item_id) REFERENCES order_items(id);
CREATE INDEX services_order_item_id_idx ON services (order_item_id);

-- invoices ---------------------------------------------------------------------------
CREATE TABLE invoices (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_number TEXT          NOT NULL UNIQUE,
    client_id      BIGINT        NOT NULL REFERENCES clients(id),
    status         TEXT          NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft','unpaid','paid','overdue','cancelled','refunded')),
    subtotal       BIGINT        NOT NULL DEFAULT 0,
    discount       BIGINT        NOT NULL DEFAULT 0,
    tax_rate       NUMERIC(5,2)  NOT NULL DEFAULT 0,
    tax_total      BIGINT        NOT NULL DEFAULT 0,
    credit_applied BIGINT        NOT NULL DEFAULT 0,
    total          BIGINT        NOT NULL DEFAULT 0,
    currency       TEXT          NOT NULL DEFAULT 'IDR',
    due_date       DATE          NOT NULL,
    paid_at        TIMESTAMPTZ   NULL,
    notes          TEXT          NOT NULL DEFAULT '',
    pdf_object_key TEXT          NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX invoices_client_id_idx ON invoices (client_id);
CREATE INDEX invoices_status_due_date_idx ON invoices (status, due_date);
CREATE TRIGGER invoices_updated_at BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- invoice_items -----------------------------------------------------------------------
CREATE TABLE invoice_items (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id   BIGINT      NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description  TEXT        NOT NULL DEFAULT '',
    amount       BIGINT      NOT NULL DEFAULT 0,
    taxed        BOOLEAN     NOT NULL DEFAULT TRUE,
    related_type TEXT        NOT NULL DEFAULT 'manual'
                 CHECK (related_type IN ('order_item','service_renewal','domain_renewal','late_fee','deposit','manual')),
    related_id   BIGINT      NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX invoice_items_invoice_id_idx ON invoice_items (invoice_id);
CREATE TRIGGER invoice_items_updated_at BEFORE UPDATE ON invoice_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- transactions --------------------------------------------------------------------------
CREATE TABLE transactions (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id        BIGINT      NOT NULL REFERENCES invoices(id),
    gateway           TEXT        NOT NULL CHECK (gateway IN ('duitku','credit','manual')),
    method_code       TEXT        NOT NULL DEFAULT '',
    merchant_order_id TEXT        NULL UNIQUE,
    gateway_reference TEXT        NOT NULL DEFAULT '',
    amount            BIGINT      NOT NULL DEFAULT 0,
    fee               BIGINT      NOT NULL DEFAULT 0,
    status            TEXT        NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','success','failed','expired','refunded')),
    raw               JSONB       NOT NULL DEFAULT '{}'::jsonb,
    paid_at           TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX transactions_invoice_id_idx ON transactions (invoice_id);
CREATE INDEX transactions_status_idx ON transactions (status);
-- idempotent payment activation: at most ONE successful transaction per invoice
CREATE UNIQUE INDEX transactions_one_success_per_invoice_uniq
    ON transactions (invoice_id) WHERE status = 'success';
CREATE TRIGGER transactions_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- credit_ledger ---------------------------------------------------------------------------
CREATE TABLE credit_ledger (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id          BIGINT      NOT NULL REFERENCES clients(id),
    delta              BIGINT      NOT NULL,
    balance_after      BIGINT      NOT NULL,
    reason             TEXT        NOT NULL DEFAULT '',
    related_invoice_id BIGINT      NULL REFERENCES invoices(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX credit_ledger_client_id_idx ON credit_ledger (client_id);
CREATE INDEX credit_ledger_related_invoice_id_idx ON credit_ledger (related_invoice_id);
CREATE TRIGGER credit_ledger_updated_at BEFORE UPDATE ON credit_ledger
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- servers ------------------------------------------------------------------------------------
CREATE TABLE servers (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id      BIGINT      NULL REFERENCES server_groups(id),
    name          TEXT        NOT NULL,
    module        TEXT        NOT NULL CHECK (module IN ('cpanel','directadmin','none')),
    hostname      TEXT        NOT NULL,
    port          INTEGER     NOT NULL DEFAULT 2087,
    username      TEXT        NOT NULL DEFAULT '',
    password_enc  TEXT        NOT NULL DEFAULT '',
    api_token_enc TEXT        NOT NULL DEFAULT '',
    use_ssl       BOOLEAN     NOT NULL DEFAULT TRUE,
    nameserver1   TEXT        NOT NULL DEFAULT '',
    nameserver2   TEXT        NOT NULL DEFAULT '',
    nameserver3   TEXT        NOT NULL DEFAULT '',
    nameserver4   TEXT        NOT NULL DEFAULT '',
    max_accounts  INTEGER     NOT NULL DEFAULT 0, -- 0 = unlimited
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX servers_group_id_idx ON servers (group_id);
CREATE TRIGGER servers_updated_at BEFORE UPDATE ON servers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- now services.server_id FK is resolvable
ALTER TABLE services
    ADD CONSTRAINT services_server_id_fkey
    FOREIGN KEY (server_id) REFERENCES servers(id);

-- ticket_departments ------------------------------------------------------------------------
CREATE TABLE ticket_departments (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL DEFAULT '',
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    sort       INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER ticket_departments_updated_at BEFORE UPDATE ON ticket_departments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- tickets ---------------------------------------------------------------------------------
CREATE TABLE tickets (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_number    TEXT        NOT NULL UNIQUE,
    client_id        BIGINT      NULL REFERENCES clients(id),
    department_id    BIGINT      NOT NULL REFERENCES ticket_departments(id),
    subject          TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'open'
                     CHECK (status IN ('open','answered','customer_reply','on_hold','closed')),
    priority         TEXT        NOT NULL DEFAULT 'medium'
                     CHECK (priority IN ('low','medium','high')),
    assigned_user_id BIGINT      NULL REFERENCES users(id),
    last_reply_at    TIMESTAMPTZ NULL,
    closed_at        TIMESTAMPTZ NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX tickets_client_id_idx ON tickets (client_id);
CREATE INDEX tickets_department_id_idx ON tickets (department_id);
CREATE INDEX tickets_assigned_user_id_idx ON tickets (assigned_user_id);
CREATE INDEX tickets_status_idx ON tickets (status);
CREATE TRIGGER tickets_updated_at BEFORE UPDATE ON tickets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ticket_replies ----------------------------------------------------------------------------
CREATE TABLE ticket_replies (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id   BIGINT      NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id     BIGINT      NULL REFERENCES users(id),
    author_name TEXT        NOT NULL DEFAULT '',
    message     TEXT        NOT NULL,
    is_internal BOOLEAN     NOT NULL DEFAULT FALSE,
    attachments JSONB       NOT NULL DEFAULT '[]'::jsonb, -- [{object_key,filename,size,content_type}]
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_replies_ticket_id_idx ON ticket_replies (ticket_id);
CREATE INDEX ticket_replies_user_id_idx ON ticket_replies (user_id);
CREATE TRIGGER ticket_replies_updated_at BEFORE UPDATE ON ticket_replies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- email_templates -----------------------------------------------------------------------------
CREATE TABLE email_templates (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT        NOT NULL,
    locale     TEXT        NOT NULL DEFAULT 'id',
    subject    TEXT        NOT NULL,
    body_html  TEXT        NOT NULL DEFAULT '',
    body_text  TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (key, locale)
);
CREATE TRIGGER email_templates_updated_at BEFORE UPDATE ON email_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- email_log ------------------------------------------------------------------------------------
CREATE TABLE email_log (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    to_email     TEXT        NOT NULL,
    template_key TEXT        NOT NULL DEFAULT '',
    subject      TEXT        NOT NULL DEFAULT '',
    status       TEXT        NOT NULL DEFAULT 'queued'
                 CHECK (status IN ('queued','sent','failed')),
    error        TEXT        NOT NULL DEFAULT '',
    sent_at      TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX email_log_to_email_idx ON email_log (to_email);
CREATE INDEX email_log_status_idx ON email_log (status);
CREATE TRIGGER email_log_updated_at BEFORE UPDATE ON email_log
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- settings --------------------------------------------------------------------------------------
CREATE TABLE settings (
    key   TEXT  PRIMARY KEY,
    value JSONB NOT NULL
);

-- counters: invoice/order/ticket numbering via UPDATE ... RETURNING inside tx --------------------
CREATE TABLE counters (
    scope TEXT   PRIMARY KEY,
    value BIGINT NOT NULL DEFAULT 0
);

-- audit_logs --------------------------------------------------------------------------------------
CREATE TABLE audit_logs (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT      NULL REFERENCES users(id),
    action     TEXT        NOT NULL,
    entity     TEXT        NOT NULL DEFAULT '',
    entity_id  BIGINT      NOT NULL DEFAULT 0,
    before     JSONB       NULL,
    after      JSONB       NULL,
    ip         TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_user_id_idx ON audit_logs (user_id);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity, entity_id);

-- integration_logs ----------------------------------------------------------------------------------
CREATE TABLE integration_logs (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider    TEXT        NOT NULL,
    endpoint    TEXT        NOT NULL DEFAULT '',
    method      TEXT        NOT NULL DEFAULT '',
    status_code INTEGER     NOT NULL DEFAULT 0,
    success     BOOLEAN     NOT NULL DEFAULT FALSE,
    latency_ms  BIGINT      NOT NULL DEFAULT 0,
    request     JSONB       NULL, -- redacted
    response    JSONB       NULL, -- redacted
    error       TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX integration_logs_provider_idx ON integration_logs (provider);
CREATE INDEX integration_logs_created_at_idx ON integration_logs (created_at);
