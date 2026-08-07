-- Revert 000020: drop the shared layout and restore the original plain-HTML
-- template bodies. Guarded on the `em-h1` marker every redesigned body carries,
-- so a template edited by an operator after 000020 ran is left untouched.

DELETE FROM email_templates WHERE key = '_layout';

-- verify_email ----------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Verifikasi email Anda',
    body_html = '<p>Halo {{.Name}},</p><p>Terima kasih telah mendaftar di {{.CompanyName}}. Klik tautan berikut untuk memverifikasi email Anda:</p><p><a href="{{.VerifyURL}}">Verifikasi Email</a></p><p>Tautan berlaku 24 jam.</p>',
    body_text = 'Halo {{.Name}}, verifikasi email Anda di: {{.VerifyURL}}'
    WHERE key = 'verify_email' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Verify your email',
    body_html = '<p>Hello {{.Name}},</p><p>Thanks for signing up at {{.CompanyName}}. Click the link below to verify your email:</p><p><a href="{{.VerifyURL}}">Verify Email</a></p><p>The link is valid for 24 hours.</p>',
    body_text = 'Hello {{.Name}}, verify your email at: {{.VerifyURL}}'
    WHERE key = 'verify_email' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- reset_password --------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Atur ulang kata sandi',
    body_html = '<p>Halo {{.Name}},</p><p>Kami menerima permintaan untuk mengatur ulang kata sandi akun Anda. Klik tautan berikut:</p><p><a href="{{.ResetURL}}">Atur Ulang Kata Sandi</a></p><p>Abaikan email ini jika Anda tidak meminta pengaturan ulang.</p>',
    body_text = 'Halo {{.Name}}, atur ulang kata sandi Anda di: {{.ResetURL}}'
    WHERE key = 'reset_password' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Reset your password',
    body_html = '<p>Hello {{.Name}},</p><p>We received a request to reset your account password. Click the link below:</p><p><a href="{{.ResetURL}}">Reset Password</a></p><p>Ignore this email if you did not request a reset.</p>',
    body_text = 'Hello {{.Name}}, reset your password at: {{.ResetURL}}'
    WHERE key = 'reset_password' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- invoice_created -------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Tagihan baru {{.InvoiceNumber}}',
    body_html = '<p>Halo {{.Name}},</p><p>Tagihan baru <strong>{{.InvoiceNumber}}</strong> sebesar <strong>{{.Total}}</strong> telah diterbitkan. Jatuh tempo {{.DueDate}}.</p><p><a href="{{.InvoiceURL}}">Lihat &amp; Bayar Tagihan</a></p>',
    body_text = 'Halo {{.Name}}, tagihan {{.InvoiceNumber}} sebesar {{.Total}} jatuh tempo {{.DueDate}}. Bayar di: {{.InvoiceURL}}'
    WHERE key = 'invoice_created' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'New invoice {{.InvoiceNumber}}',
    body_html = '<p>Hello {{.Name}},</p><p>A new invoice <strong>{{.InvoiceNumber}}</strong> for <strong>{{.Total}}</strong> has been issued. Due {{.DueDate}}.</p><p><a href="{{.InvoiceURL}}">View &amp; Pay Invoice</a></p>',
    body_text = 'Hello {{.Name}}, invoice {{.InvoiceNumber}} for {{.Total}} is due {{.DueDate}}. Pay at: {{.InvoiceURL}}'
    WHERE key = 'invoice_created' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- invoice_reminder ------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Pengingat: tagihan {{.InvoiceNumber}} jatuh tempo {{.DueDate}}',
    body_html = '<p>Halo {{.Name}},</p><p>Tagihan <strong>{{.InvoiceNumber}}</strong> sebesar <strong>{{.Total}}</strong> akan jatuh tempo pada <strong>{{.DueDate}}</strong>.</p><p><a href="{{.InvoiceURL}}">Bayar Sekarang</a></p>',
    body_text = 'Halo {{.Name}}, tagihan {{.InvoiceNumber}} sebesar {{.Total}} jatuh tempo {{.DueDate}}. Bayar di: {{.InvoiceURL}}'
    WHERE key = 'invoice_reminder' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Reminder: invoice {{.InvoiceNumber}} due {{.DueDate}}',
    body_html = '<p>Hello {{.Name}},</p><p>Invoice <strong>{{.InvoiceNumber}}</strong> for <strong>{{.Total}}</strong> is due on <strong>{{.DueDate}}</strong>.</p><p><a href="{{.InvoiceURL}}">Pay Now</a></p>',
    body_text = 'Hello {{.Name}}, invoice {{.InvoiceNumber}} for {{.Total}} is due {{.DueDate}}. Pay at: {{.InvoiceURL}}'
    WHERE key = 'invoice_reminder' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- invoice_overdue -------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Tagihan {{.InvoiceNumber}} telah lewat jatuh tempo',
    body_html = '<p>Halo {{.Name}},</p><p>Tagihan <strong>{{.InvoiceNumber}}</strong> sebesar <strong>{{.Total}}</strong> telah melewati jatuh tempo ({{.DueDate}}). Layanan Anda berisiko ditangguhkan.</p><p><a href="{{.InvoiceURL}}">Bayar Sekarang</a></p>',
    body_text = 'Halo {{.Name}}, tagihan {{.InvoiceNumber}} sebesar {{.Total}} lewat jatuh tempo. Bayar di: {{.InvoiceURL}}'
    WHERE key = 'invoice_overdue' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Invoice {{.InvoiceNumber}} is overdue',
    body_html = '<p>Hello {{.Name}},</p><p>Invoice <strong>{{.InvoiceNumber}}</strong> for <strong>{{.Total}}</strong> is past its due date ({{.DueDate}}). Your services are at risk of suspension.</p><p><a href="{{.InvoiceURL}}">Pay Now</a></p>',
    body_text = 'Hello {{.Name}}, invoice {{.InvoiceNumber}} for {{.Total}} is overdue. Pay at: {{.InvoiceURL}}'
    WHERE key = 'invoice_overdue' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- payment_received ------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Pembayaran diterima untuk {{.InvoiceNumber}}',
    body_html = '<p>Halo {{.Name}},</p><p>Pembayaran sebesar <strong>{{.Amount}}</strong> untuk tagihan <strong>{{.InvoiceNumber}}</strong> telah kami terima. Terima kasih!</p>',
    body_text = 'Halo {{.Name}}, pembayaran {{.Amount}} untuk {{.InvoiceNumber}} telah diterima. Terima kasih!'
    WHERE key = 'payment_received' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Payment received for {{.InvoiceNumber}}',
    body_html = '<p>Hello {{.Name}},</p><p>We received your payment of <strong>{{.Amount}}</strong> for invoice <strong>{{.InvoiceNumber}}</strong>. Thank you!</p>',
    body_text = 'Hello {{.Name}}, payment of {{.Amount}} for {{.InvoiceNumber}} received. Thank you!'
    WHERE key = 'payment_received' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- service_activated -----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan Anda aktif: {{.ServiceName}}',
    body_html = '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> untuk domain <strong>{{.Domain}}</strong> telah aktif.</p><p>Username: <strong>{{.Username}}</strong><br>Password: <strong>{{.Password}}</strong><br>Panel: <a href="{{.PanelURL}}">{{.PanelURL}}</a></p>',
    body_text = 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) aktif. Username: {{.Username}}, Password: {{.Password}}, Panel: {{.PanelURL}}'
    WHERE key = 'service_activated' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Your service is active: {{.ServiceName}}',
    body_html = '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> for domain <strong>{{.Domain}}</strong> is now active.</p><p>Username: <strong>{{.Username}}</strong><br>Password: <strong>{{.Password}}</strong><br>Panel: <a href="{{.PanelURL}}">{{.PanelURL}}</a></p>',
    body_text = 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) is active. Username: {{.Username}}, Password: {{.Password}}, Panel: {{.PanelURL}}'
    WHERE key = 'service_activated' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- service_suspended -----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan ditangguhkan: {{.ServiceName}}',
    body_html = '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah ditangguhkan.</p><p>Alasan: {{.Reason}}</p><p>Segera lunasi tagihan Anda untuk mengaktifkan kembali layanan.</p>',
    body_text = 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) ditangguhkan. Alasan: {{.Reason}}'
    WHERE key = 'service_suspended' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Service suspended: {{.ServiceName}}',
    body_html = '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been suspended.</p><p>Reason: {{.Reason}}</p><p>Please settle your outstanding invoice to restore the service.</p>',
    body_text = 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) suspended. Reason: {{.Reason}}'
    WHERE key = 'service_suspended' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- service_unsuspended ---------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan diaktifkan kembali: {{.ServiceName}}',
    body_html = '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah diaktifkan kembali. Terima kasih atas pembayarannya.</p>',
    body_text = 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) telah aktif kembali.'
    WHERE key = 'service_unsuspended' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Service restored: {{.ServiceName}}',
    body_html = '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been restored. Thank you for your payment.</p>',
    body_text = 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) has been restored.'
    WHERE key = 'service_unsuspended' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- service_terminated ----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan dihentikan: {{.ServiceName}}',
    body_html = '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah dihentikan permanen.</p>',
    body_text = 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) telah dihentikan.'
    WHERE key = 'service_terminated' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Service terminated: {{.ServiceName}}',
    body_html = '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been terminated.</p>',
    body_text = 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) has been terminated.'
    WHERE key = 'service_terminated' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- service_renewed -------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan diperpanjang: {{.ServiceName}}',
    body_html = '<p>Halo {{.Name}},</p><p>Layanan <strong>{{.ServiceName}}</strong> ({{.Domain}}) telah diperpanjang. Jatuh tempo berikutnya: <strong>{{.NextDueDate}}</strong>.</p>',
    body_text = 'Halo {{.Name}}, layanan {{.ServiceName}} ({{.Domain}}) diperpanjang. Jatuh tempo berikutnya: {{.NextDueDate}}.'
    WHERE key = 'service_renewed' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Service renewed: {{.ServiceName}}',
    body_html = '<p>Hello {{.Name}},</p><p>Your service <strong>{{.ServiceName}}</strong> ({{.Domain}}) has been renewed. Next due date: <strong>{{.NextDueDate}}</strong>.</p>',
    body_text = 'Hello {{.Name}}, service {{.ServiceName}} ({{.Domain}}) has been renewed. Next due date: {{.NextDueDate}}.'
    WHERE key = 'service_renewed' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- domain_registered -----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Domain terdaftar: {{.Domain}}',
    body_html = '<p>Halo {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> berhasil didaftarkan. Berlaku hingga <strong>{{.ExpiryDate}}</strong>.</p>',
    body_text = 'Halo {{.Name}}, domain {{.Domain}} terdaftar hingga {{.ExpiryDate}}.'
    WHERE key = 'domain_registered' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Domain registered: {{.Domain}}',
    body_html = '<p>Hello {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> has been registered successfully. Expires on <strong>{{.ExpiryDate}}</strong>.</p>',
    body_text = 'Hello {{.Name}}, domain {{.Domain}} registered until {{.ExpiryDate}}.'
    WHERE key = 'domain_registered' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- domain_renewed --------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Domain diperpanjang: {{.Domain}}',
    body_html = '<p>Halo {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> berhasil diperpanjang. Masa berlaku baru hingga <strong>{{.ExpiryDate}}</strong>.</p>',
    body_text = 'Halo {{.Name}}, domain {{.Domain}} diperpanjang hingga {{.ExpiryDate}}.'
    WHERE key = 'domain_renewed' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Domain renewed: {{.Domain}}',
    body_html = '<p>Hello {{.Name}},</p><p>Domain <strong>{{.Domain}}</strong> has been renewed. New expiry date: <strong>{{.ExpiryDate}}</strong>.</p>',
    body_text = 'Hello {{.Name}}, domain {{.Domain}} renewed until {{.ExpiryDate}}.'
    WHERE key = 'domain_renewed' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- ticket_opened ---------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Tiket dibuka: {{.TicketNumber}} - {{.Subject}}',
    body_html = '<p>Halo {{.Name}},</p><p>Tiket <strong>{{.TicketNumber}}</strong> dengan subjek "<em>{{.Subject}}</em>" telah dibuka. Tim kami akan segera merespons.</p><p><a href="{{.TicketURL}}">Lihat Tiket</a></p>',
    body_text = 'Halo {{.Name}}, tiket {{.TicketNumber}} ({{.Subject}}) telah dibuka. Lihat di: {{.TicketURL}}'
    WHERE key = 'ticket_opened' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'Ticket opened: {{.TicketNumber}} - {{.Subject}}',
    body_html = '<p>Hello {{.Name}},</p><p>Ticket <strong>{{.TicketNumber}}</strong> with subject "<em>{{.Subject}}</em>" has been opened. Our team will respond shortly.</p><p><a href="{{.TicketURL}}">View Ticket</a></p>',
    body_text = 'Hello {{.Name}}, ticket {{.TicketNumber}} ({{.Subject}}) opened. View at: {{.TicketURL}}'
    WHERE key = 'ticket_opened' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- ticket_replied --------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Balasan baru pada tiket {{.TicketNumber}}',
    body_html = '<p>Halo {{.Name}},</p><p>Ada balasan baru pada tiket <strong>{{.TicketNumber}}</strong> ({{.Subject}}).</p><p><a href="{{.TicketURL}}">Lihat Balasan</a></p>',
    body_text = 'Halo {{.Name}}, ada balasan baru pada tiket {{.TicketNumber}}. Lihat di: {{.TicketURL}}'
    WHERE key = 'ticket_replied' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = 'New reply on ticket {{.TicketNumber}}',
    body_html = '<p>Hello {{.Name}},</p><p>There is a new reply on ticket <strong>{{.TicketNumber}}</strong> ({{.Subject}}).</p><p><a href="{{.TicketURL}}">View Reply</a></p>',
    body_text = 'Hello {{.Name}}, new reply on ticket {{.TicketNumber}}. View at: {{.TicketURL}}'
    WHERE key = 'ticket_replied' AND locale = 'en' AND body_html LIKE '%em-h1%';

-- admin_alert -----------------------------------------------------------------
UPDATE email_templates SET
    subject   = '[WHCMS] Peringatan: {{.Subject}}',
    body_html = '<p>Peringatan sistem WHCMS:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
    body_text = 'Peringatan WHCMS: {{.Subject}} - {{.Detail}}'
    WHERE key = 'admin_alert' AND locale = 'id' AND body_html LIKE '%em-h1%';
UPDATE email_templates SET
    subject   = '[WHCMS] Alert: {{.Subject}}',
    body_html = '<p>WHCMS system alert:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>',
    body_text = 'WHCMS alert: {{.Subject}} - {{.Detail}}'
    WHERE key = 'admin_alert' AND locale = 'en' AND body_html LIKE '%em-h1%';
