-- Portal features: announcements, knowledgebase (categories + articles),
-- network status (issues) and guest ticket support. Enums are TEXT + CHECK
-- constraints matching domain enum values exactly.

-- announcements ---------------------------------------------------------------
CREATE TABLE announcements (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title        TEXT        NOT NULL,
    slug         TEXT        NOT NULL UNIQUE,
    body         TEXT        NOT NULL DEFAULT '',
    published    BOOLEAN     NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ NULL,
    author_id    BIGINT      NULL REFERENCES users(id),
    deleted_at   TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX announcements_published_idx ON announcements (published, published_at DESC) WHERE deleted_at IS NULL;
CREATE TRIGGER announcements_updated_at BEFORE UPDATE ON announcements
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- kb_categories ---------------------------------------------------------------
CREATE TABLE kb_categories (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    sort        INTEGER     NOT NULL DEFAULT 0,
    hidden      BOOLEAN     NOT NULL DEFAULT FALSE,
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER kb_categories_updated_at BEFORE UPDATE ON kb_categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- kb_articles -----------------------------------------------------------------
CREATE TABLE kb_articles (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id BIGINT      NOT NULL REFERENCES kb_categories(id),
    title       TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    body        TEXT        NOT NULL DEFAULT '',
    published   BOOLEAN     NOT NULL DEFAULT FALSE,
    views       BIGINT      NOT NULL DEFAULT 0,
    sort        INTEGER     NOT NULL DEFAULT 0,
    author_id   BIGINT      NULL REFERENCES users(id),
    deleted_at  TIMESTAMPTZ NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX kb_articles_category_id_idx ON kb_articles (category_id);
CREATE INDEX kb_articles_published_idx ON kb_articles (published) WHERE deleted_at IS NULL;
CREATE TRIGGER kb_articles_updated_at BEFORE UPDATE ON kb_articles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- network_issues --------------------------------------------------------------
CREATE TABLE network_issues (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL DEFAULT '',
    type       TEXT        NOT NULL DEFAULT 'issue'
               CHECK (type IN ('scheduled','issue','outage')),
    severity   TEXT        NOT NULL DEFAULT 'minor'
               CHECK (severity IN ('minor','major','critical')),
    status     TEXT        NOT NULL DEFAULT 'investigating'
               CHECK (status IN ('investigating','identified','monitoring','resolved','scheduled')),
    affected   TEXT        NOT NULL DEFAULT '',
    starts_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ends_at    TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX network_issues_status_idx ON network_issues (status) WHERE deleted_at IS NULL;
CREATE INDEX network_issues_starts_at_idx ON network_issues (starts_at DESC);
CREATE TRIGGER network_issues_updated_at BEFORE UPDATE ON network_issues
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- guest ticket support --------------------------------------------------------
ALTER TABLE tickets ADD COLUMN guest_name TEXT NULL;
ALTER TABLE tickets ADD COLUMN guest_email TEXT NULL;
