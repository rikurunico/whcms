-- Forward fix for the early "BillPanel" -> "WHCMS" product rename, which
-- hand-edited the already-applied 000002_seed_core seed data instead of
-- adding a new migration. Databases migrated before that rename still have
-- schema_migrations marking version 2 as applied, so 000002's edited INSERTs
-- never re-run and the stale "BillPanel" seed values persist. Backfill them
-- here, forward-only, guarded so it's a no-op on databases seeded post-rebrand
-- (already "WHCMS") or where an operator has since customized these values.

UPDATE settings SET value = '"WHCMS"'
    WHERE key = 'company.name' AND value = '"BillPanel"';

UPDATE settings SET value = '"WHCMS"'
    WHERE key = 'mail.from_name' AND value = '"BillPanel"';

UPDATE email_templates SET
    subject = '[WHCMS] Peringatan: {{.Subject}}',
    body_html = '<p>Peringatan sistem WHCMS:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
    body_text = 'Peringatan WHCMS: {{.Subject}} - {{.Detail}}'
    WHERE key = 'admin_alert' AND locale = 'id' AND subject = '[BillPanel] Peringatan: {{.Subject}}';

UPDATE email_templates SET
    subject = '[WHCMS] Alert: {{.Subject}}',
    body_html = '<p>WHCMS system alert:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
    body_text = 'WHCMS alert: {{.Subject}} - {{.Detail}}'
    WHERE key = 'admin_alert' AND locale = 'en' AND subject = '[BillPanel] Alert: {{.Subject}}';
