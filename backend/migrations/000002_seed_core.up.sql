-- Core seed data: default settings (§10), email templates (id+en),
-- ticket departments, rdash registrar row.

-- Settings (JSONB values; gateway/registrar secrets come from ENV only) --------
INSERT INTO settings (key, value) VALUES
    ('company.name',                 '"WHCMS"'),
    ('company.logo_key',             '""'),
    ('company.address',              '""'),
    ('company.email',                '"billing@example.com"'),
    ('billing.tax_enabled',          'false'),
    ('billing.tax_rate',             '11'),
    ('billing.tax_inclusive',        'false'),
    ('billing.invoice_due_days',     '3'),
    ('billing.renewal_lead_days',    '14'),
    ('billing.late_fee_enabled',     'false'),
    ('billing.late_fee_amount',      '0'),
    ('billing.reminder_days',        '[7,3,1]'),
    ('billing.overdue_reminder_days','[1,3,7]'),
    ('automation.suspend_after_days','7'),
    ('automation.terminate_after_days','21'),
    ('mail.from_name',               '"WHCMS"'),
    ('mail.from_email',              '"no-reply@example.com"'),
    ('tickets.allowed_extensions',   '["jpg","jpeg","png","gif","pdf","txt","zip"]'),
    ('tickets.max_attachment_mb',    '8')
ON CONFLICT (key) DO NOTHING;

-- Ticket departments -------------------------------------------------------------
INSERT INTO ticket_departments (name, email, active, sort) VALUES
    ('Support', 'support@example.com', TRUE, 1),
    ('Billing', 'billing@example.com', TRUE, 2);

-- Registrar ------------------------------------------------------------------------
INSERT INTO registrars (name, active, config) VALUES
    ('rdash', TRUE, '{}')
ON CONFLICT (name) DO NOTHING;

-- Email templates (id + en) ----------------------------------------------------------
INSERT INTO email_templates (key, locale, subject, body_html, body_text) VALUES
-- verify_email
('verify_email', 'id', 'Verifikasi email Anda',
 '<p>Halo {{.Name}},</p><p>Terima kasih telah mendaftar di {{.CompanyName}}. Klik tautan berikut untuk memverifikasi email Anda:</p><p><a href="{{.VerifyURL}}">Verifikasi Email</a></p><p>Tautan berlaku 24 jam.</p>',
 'Halo {{.Name}}, verifikasi email Anda di: {{.VerifyURL}}'),
('verify_email', 'en', 'Verify your email',
 '<p>Hello {{.Name}},</p><p>Thanks for signing up at {{.CompanyName}}. Click the link below to verify your email:</p><p><a href="{{.VerifyURL}}">Verify Email</a></p><p>The link is valid for 24 hours.</p>',
 'Hello {{.Name}}, verify your email at: {{.VerifyURL}}'),
-- reset_password
('reset_password', 'id', 'Atur ulang kata sandi',
 '<p>Halo {{.Name}},</p><p>Kami menerima permintaan untuk mengatur ulang kata sandi akun Anda. Klik tautan berikut:</p><p><a href="{{.ResetURL}}">Atur Ulang Kata Sandi</a></p><p>Abaikan email ini jika Anda tidak meminta pengaturan ulang.</p>',
 'Halo {{.Name}}, atur ulang kata sandi Anda di: {{.ResetURL}}'),
('reset_password', 'en', 'Reset your password',
 '<p>Hello {{.Name}},</p><p>We received a request to reset your account password. Click the link below:</p><p><a href="{{.ResetURL}}">Reset Password</a></p><p>Ignore this email if you did not request a reset.</p>',
 'Hello {{.Name}}, reset your password at: {{.ResetURL}}'),
-- invoice_created
('invoice_created', 'id', 'Tagihan baru {{.InvoiceNumber}}',
 '<p>Halo {{.Name}},</p><p>Tagihan baru <strong>{{.InvoiceNumber}}</strong> sebesar <strong>{{.Total}}</strong> telah diterbitkan. Jatuh tempo {{.DueDate}}.</p><p><a href="{{.InvoiceURL}}">Lihat &amp; Bayar Tagihan</a></p>',
 'Halo {{.Name}}, tagihan {{.InvoiceNumber}} sebesar {{.Total}} jatuh tempo {{.DueDate}}. Bayar di: {{.InvoiceURL}}'),
('invoice_created', 'en', 'New invoice {{.InvoiceNumber}}',
 '<p>Hello {{.Name}},</p><p>A new invoice <strong>{{.InvoiceNumber}}</strong> for <strong>{{.Total}}</strong> has been issued. Due {{.DueDate}}.</p><p><a href="{{.InvoiceURL}}">View &amp; Pay Invoice</a></p>',
 'Hello {{.Name}}, invoice {{.InvoiceNumber}} for {{.Total}} is due {{.DueDate}}. Pay at: {{.InvoiceURL}}'),
-- payment_received
('payment_received', 'id', 'Pembayaran diterima untuk {{.InvoiceNumber}}',
 '<p>Halo {{.Name}},</p><p>Pembayaran sebesar <strong>{{.Amount}}</strong> untuk tagihan <strong>{{.InvoiceNumber}}</strong> telah kami terima. Terima kasih!</p>',
 'Halo {{.Name}}, pembayaran {{.Amount}} untuk {{.InvoiceNumber}} telah diterima. Terima kasih!'),
('payment_received', 'en', 'Payment received for {{.InvoiceNumber}}',
 '<p>Hello {{.Name}},</p><p>We received your payment of <strong>{{.Amount}}</strong> for invoice <strong>{{.InvoiceNumber}}</strong>. Thank you!</p>',
 'Hello {{.Name}}, payment of {{.Amount}} for {{.InvoiceNumber}} received. Thank you!'),
-- service_activated
('service_activated', 'id', 'Layanan Anda aktif: {{.ServiceName}}',
 '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> untuk domain <strong>{{.Domain}}</strong> telah aktif.</p><p>Username: <strong>{{.Username}}</strong><br>Password: <strong>{{.Password}}</strong><br>Panel: <a href="{{.PanelURL}}">{{.PanelURL}}</a></p>',
 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) aktif. Username: {{.Username}}, Password: {{.Password}}, Panel: {{.PanelURL}}'),
('service_activated', 'en', 'Your service is active: {{.ServiceName}}',
 '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> for domain <strong>{{.Domain}}</strong> is now active.</p><p>Username: <strong>{{.Username}}</strong><br>Password: <strong>{{.Password}}</strong><br>Panel: <a href="{{.PanelURL}}">{{.PanelURL}}</a></p>',
 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) is active. Username: {{.Username}}, Password: {{.Password}}, Panel: {{.PanelURL}}'),
-- service_suspended
('service_suspended', 'id', 'Layanan ditangguhkan: {{.ServiceName}}',
 '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah ditangguhkan.</p><p>Alasan: {{.Reason}}</p><p>Segera lunasi tagihan Anda untuk mengaktifkan kembali layanan.</p>',
 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) ditangguhkan. Alasan: {{.Reason}}'),
('service_suspended', 'en', 'Service suspended: {{.ServiceName}}',
 '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been suspended.</p><p>Reason: {{.Reason}}</p><p>Please settle your outstanding invoice to restore the service.</p>',
 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) suspended. Reason: {{.Reason}}'),
-- service_unsuspended
('service_unsuspended', 'id', 'Layanan diaktifkan kembali: {{.ServiceName}}',
 '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah diaktifkan kembali. Terima kasih atas pembayarannya.</p>',
 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) telah aktif kembali.'),
('service_unsuspended', 'en', 'Service restored: {{.ServiceName}}',
 '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been restored. Thank you for your payment.</p>',
 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) has been restored.'),
-- service_terminated
('service_terminated', 'id', 'Layanan dihentikan: {{.ServiceName}}',
 '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah dihentikan permanen.</p>',
 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) telah dihentikan.'),
('service_terminated', 'en', 'Service terminated: {{.ServiceName}}',
 '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been terminated.</p>',
 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) has been terminated.'),
-- invoice_reminder
('invoice_reminder', 'id', 'Pengingat: tagihan {{.InvoiceNumber}} jatuh tempo {{.DueDate}}',
 '<p>Halo {{.Name}},</p><p>Tagihan <strong>{{.InvoiceNumber}}</strong> sebesar <strong>{{.Total}}</strong> akan jatuh tempo pada <strong>{{.DueDate}}</strong>.</p><p><a href="{{.InvoiceURL}}">Bayar Sekarang</a></p>',
 'Halo {{.Name}}, tagihan {{.InvoiceNumber}} sebesar {{.Total}} jatuh tempo {{.DueDate}}. Bayar di: {{.InvoiceURL}}'),
('invoice_reminder', 'en', 'Reminder: invoice {{.InvoiceNumber}} due {{.DueDate}}',
 '<p>Hello {{.Name}},</p><p>Invoice <strong>{{.InvoiceNumber}}</strong> for <strong>{{.Total}}</strong> is due on <strong>{{.DueDate}}</strong>.</p><p><a href="{{.InvoiceURL}}">Pay Now</a></p>',
 'Hello {{.Name}}, invoice {{.InvoiceNumber}} for {{.Total}} is due {{.DueDate}}. Pay at: {{.InvoiceURL}}'),
-- invoice_overdue
('invoice_overdue', 'id', 'Tagihan {{.InvoiceNumber}} telah lewat jatuh tempo',
 '<p>Halo {{.Name}},</p><p>Tagihan <strong>{{.InvoiceNumber}}</strong> sebesar <strong>{{.Total}}</strong> telah melewati jatuh tempo ({{.DueDate}}). Layanan Anda berisiko ditangguhkan.</p><p><a href="{{.InvoiceURL}}">Bayar Sekarang</a></p>',
 'Halo {{.Name}}, tagihan {{.InvoiceNumber}} sebesar {{.Total}} lewat jatuh tempo. Bayar di: {{.InvoiceURL}}'),
('invoice_overdue', 'en', 'Invoice {{.InvoiceNumber}} is overdue',
 '<p>Hello {{.Name}},</p><p>Invoice <strong>{{.InvoiceNumber}}</strong> for <strong>{{.Total}}</strong> is past its due date ({{.DueDate}}). Your services are at risk of suspension.</p><p><a href="{{.InvoiceURL}}">Pay Now</a></p>',
 'Hello {{.Name}}, invoice {{.InvoiceNumber}} for {{.Total}} is overdue. Pay at: {{.InvoiceURL}}'),
-- domain_registered
('domain_registered', 'id', 'Domain terdaftar: {{.Domain}}',
 '<p>Halo {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> berhasil didaftarkan. Berlaku hingga <strong>{{.ExpiryDate}}</strong>.</p>',
 'Halo {{.Name}}, domain {{.Domain}} terdaftar hingga {{.ExpiryDate}}.'),
('domain_registered', 'en', 'Domain registered: {{.Domain}}',
 '<p>Hello {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> has been registered successfully. Expires on <strong>{{.ExpiryDate}}</strong>.</p>',
 'Hello {{.Name}}, domain {{.Domain}} registered until {{.ExpiryDate}}.'),
-- domain_renewed
('domain_renewed', 'id', 'Domain diperpanjang: {{.Domain}}',
 '<p>Halo {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> berhasil diperpanjang. Masa berlaku baru hingga <strong>{{.ExpiryDate}}</strong>.</p>',
 'Halo {{.Name}}, domain {{.Domain}} diperpanjang hingga {{.ExpiryDate}}.'),
('domain_renewed', 'en', 'Domain renewed: {{.Domain}}',
 '<p>Hello {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> has been renewed. New expiry date: <strong>{{.ExpiryDate}}</strong>.</p>',
 'Hello {{.Name}}, domain {{.Domain}} renewed until {{.ExpiryDate}}.'),
-- ticket_opened
('ticket_opened', 'id', 'Tiket dibuka: {{.TicketNumber}} - {{.Subject}}',
 '<p>Halo {{.Name}},</p><p>Tiket <strong>{{.TicketNumber}}</strong> dengan subjek "<em>{{.Subject}}</em>" telah dibuka. Tim kami akan segera merespons.</p><p><a href="{{.TicketURL}}">Lihat Tiket</a></p>',
 'Halo {{.Name}}, tiket {{.TicketNumber}} ({{.Subject}}) telah dibuka. Lihat di: {{.TicketURL}}'),
('ticket_opened', 'en', 'Ticket opened: {{.TicketNumber}} - {{.Subject}}',
 '<p>Hello {{.Name}},</p><p>Ticket <strong>{{.TicketNumber}}</strong> with subject "<em>{{.Subject}}</em>" has been opened. Our team will respond shortly.</p><p><a href="{{.TicketURL}}">View Ticket</a></p>',
 'Hello {{.Name}}, ticket {{.TicketNumber}} ({{.Subject}}) opened. View at: {{.TicketURL}}'),
-- ticket_replied
('ticket_replied', 'id', 'Balasan baru pada tiket {{.TicketNumber}}',
 '<p>Halo {{.Name}},</p><p>Ada balasan baru pada tiket <strong>{{.TicketNumber}}</strong> ({{.Subject}}).</p><p><a href="{{.TicketURL}}">Lihat Balasan</a></p>',
 'Halo {{.Name}}, ada balasan baru pada tiket {{.TicketNumber}}. Lihat di: {{.TicketURL}}'),
('ticket_replied', 'en', 'New reply on ticket {{.TicketNumber}}',
 '<p>Hello {{.Name}},</p><p>There is a new reply on ticket <strong>{{.TicketNumber}}</strong> ({{.Subject}}).</p><p><a href="{{.TicketURL}}">View Reply</a></p>',
 'Hello {{.Name}}, new reply on ticket {{.TicketNumber}}. View at: {{.TicketURL}}'),
-- admin_alert
('admin_alert', 'id', '[WHCMS] Peringatan: {{.Subject}}',
 '<p>Peringatan sistem WHCMS:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
 'Peringatan WHCMS: {{.Subject}} - {{.Detail}}'),
('admin_alert', 'en', '[WHCMS] Alert: {{.Subject}}',
 '<p>WHCMS system alert:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
 'WHCMS alert: {{.Subject}} - {{.Detail}}')
ON CONFLICT (key, locale) DO NOTHING;
