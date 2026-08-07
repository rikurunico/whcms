UPDATE settings SET value = '"BillPanel"'
    WHERE key = 'company.name' AND value = '"WHCMS"';

UPDATE settings SET value = '"BillPanel"'
    WHERE key = 'mail.from_name' AND value = '"WHCMS"';

UPDATE email_templates SET
    subject = '[BillPanel] Peringatan: {{.Subject}}',
    body_html = '<p>Peringatan sistem BillPanel:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
    body_text = 'Peringatan BillPanel: {{.Subject}} - {{.Detail}}'
    WHERE key = 'admin_alert' AND locale = 'id' AND subject = '[WHCMS] Peringatan: {{.Subject}}';

UPDATE email_templates SET
    subject = '[BillPanel] Alert: {{.Subject}}',
    body_html = '<p>BillPanel system alert:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
    body_text = 'BillPanel alert: {{.Subject}} - {{.Detail}}'
    WHERE key = 'admin_alert' AND locale = 'en' AND subject = '[WHCMS] Alert: {{.Subject}}';
