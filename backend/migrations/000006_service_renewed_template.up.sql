-- Add the service_renewed email template (id + en): sent when a hosting
-- service's recurring billing cycle renews. Mirrors the neighboring
-- service_unsuspended template in 000002_seed_core (same {{.Name}} /
-- {{.ServiceName}} / {{.Domain}} placeholders, plus {{.NextDueDate}} for the
-- freshly-advanced due date). 000002 is already applied elsewhere and must
-- not be edited (forward-only migrations per CLAUDE.md).
INSERT INTO email_templates (key, locale, subject, body_html, body_text) VALUES
('service_renewed', 'id', 'Layanan diperpanjang: {{.ServiceName}}',
 '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah diperpanjang. Jatuh tempo berikutnya: <strong>{{.NextDueDate}}</strong>.</p>',
 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) diperpanjang. Jatuh tempo berikutnya: {{.NextDueDate}}.'),
('service_renewed', 'en', 'Service renewed: {{.ServiceName}}',
 '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been renewed. Next due date: <strong>{{.NextDueDate}}</strong>.</p>',
 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) has been renewed. Next due date: {{.NextDueDate}}.')
ON CONFLICT (key, locale) DO NOTHING;
