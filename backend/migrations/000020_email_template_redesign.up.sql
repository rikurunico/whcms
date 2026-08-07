-- Redesign the default email templates (FR-NOTIF-002): replace the bare
-- <p> fragments seeded by 000002/000006 with a professional, responsive,
-- table-based HTML design, and introduce the shared "_layout" wrapper that
-- notifications.Service renders around every body_html (branded header,
-- support footer, mobile breakpoints).
--
-- Forward-only and non-destructive: each UPDATE is guarded by
-- `updated_at = created_at`, so a template an operator has already customized
-- is left exactly as they saved it. (admin_alert additionally matches the
-- body 000004 rewrote, which bumped updated_at without an operator edit.)

-- The global layout ------------------------------------------------------------------
INSERT INTO email_templates (key, locale, subject, body_html, body_text) VALUES
('_layout', 'id', 'Layout email global',
 '<!DOCTYPE html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="x-apple-disable-message-reformatting"><title>{{.CompanyName}}</title>
<style>
body{margin:0;padding:0;width:100%;background:#f1f1f1;}
table{border-collapse:collapse;}
img{border:0;outline:none;text-decoration:none;max-width:100%;}
@media only screen and (max-width:620px){
.em-card{width:100%!important;}
.em-pad{padding-left:22px!important;padding-right:22px!important;}
.em-btn{width:100%!important;}
.em-btn td,.em-btn a{display:block!important;text-align:center!important;}
.em-h1{font-size:20px!important;}
.em-kv td{display:block!important;width:100%!important;border-bottom:0!important;padding-bottom:0!important;}
.em-kv td.em-v{padding-top:2px!important;padding-bottom:12px!important;}
}
</style></head>
<body style="margin:0;padding:0;background:#f1f1f1;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#f1f1f1;">
<tr><td align="center" style="padding:26px 12px 34px;">
<table role="presentation" class="em-card" width="600" cellpadding="0" cellspacing="0" border="0" style="width:600px;max-width:600px;background:#ffffff;border:1px solid #e3e3e3;border-radius:6px;">
<tr><td class="em-pad" style="background:#336699;border-radius:5px 5px 0 0;padding:20px 32px;"><span style="font:600 19px/1.25 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;letter-spacing:.2px;">{{.CompanyName}}</span></td></tr>
<tr><td class="em-pad" style="padding:30px 32px 34px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">{{.Content}}</td></tr>
<tr><td class="em-pad" style="background:#fafafa;border-top:1px solid #ececec;border-radius:0 0 5px 5px;padding:18px 32px;font:400 12px/1.7 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Email ini dikirim otomatis oleh sistem {{.CompanyName}}. Mohon tidak membalas email ini.<br>Butuh bantuan? Buka <a href="{{.FrontendURL}}/support" style="color:#336699;text-decoration:none;">tiket dukungan</a>{{if .CompanyEmail}} atau hubungi <a href="mailto:{{.CompanyEmail}}" style="color:#336699;text-decoration:none;">{{.CompanyEmail}}</a>{{end}}.</td></tr>
</table>
<table role="presentation" class="em-card" width="600" cellpadding="0" cellspacing="0" border="0" style="width:600px;max-width:600px;"><tr><td align="center" style="padding:16px 24px 0;font:400 11px/1.7 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#9b9b9b;">&copy; {{.Year}} {{.CompanyName}}{{if .CompanyAddress}}<br>{{.CompanyAddress}}{{end}}</td></tr></table>
</td></tr></table>
</body></html>',
 ''),
('_layout', 'en', 'Global email layout',
 '<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="x-apple-disable-message-reformatting"><title>{{.CompanyName}}</title>
<style>
body{margin:0;padding:0;width:100%;background:#f1f1f1;}
table{border-collapse:collapse;}
img{border:0;outline:none;text-decoration:none;max-width:100%;}
@media only screen and (max-width:620px){
.em-card{width:100%!important;}
.em-pad{padding-left:22px!important;padding-right:22px!important;}
.em-btn{width:100%!important;}
.em-btn td,.em-btn a{display:block!important;text-align:center!important;}
.em-h1{font-size:20px!important;}
.em-kv td{display:block!important;width:100%!important;border-bottom:0!important;padding-bottom:0!important;}
.em-kv td.em-v{padding-top:2px!important;padding-bottom:12px!important;}
}
</style></head>
<body style="margin:0;padding:0;background:#f1f1f1;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#f1f1f1;">
<tr><td align="center" style="padding:26px 12px 34px;">
<table role="presentation" class="em-card" width="600" cellpadding="0" cellspacing="0" border="0" style="width:600px;max-width:600px;background:#ffffff;border:1px solid #e3e3e3;border-radius:6px;">
<tr><td class="em-pad" style="background:#336699;border-radius:5px 5px 0 0;padding:20px 32px;"><span style="font:600 19px/1.25 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;letter-spacing:.2px;">{{.CompanyName}}</span></td></tr>
<tr><td class="em-pad" style="padding:30px 32px 34px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">{{.Content}}</td></tr>
<tr><td class="em-pad" style="background:#fafafa;border-top:1px solid #ececec;border-radius:0 0 5px 5px;padding:18px 32px;font:400 12px/1.7 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">This message was sent automatically by {{.CompanyName}}. Please do not reply to this email.<br>Need help? Open a <a href="{{.FrontendURL}}/support" style="color:#336699;text-decoration:none;">support ticket</a>{{if .CompanyEmail}} or contact <a href="mailto:{{.CompanyEmail}}" style="color:#336699;text-decoration:none;">{{.CompanyEmail}}</a>{{end}}.</td></tr>
</table>
<table role="presentation" class="em-card" width="600" cellpadding="0" cellspacing="0" border="0" style="width:600px;max-width:600px;"><tr><td align="center" style="padding:16px 24px 0;font:400 11px/1.7 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#9b9b9b;">&copy; {{.Year}} {{.CompanyName}}{{if .CompanyAddress}}<br>{{.CompanyAddress}}{{end}}</td></tr></table>
</td></tr></table>
</body></html>',
 '')
ON CONFLICT (key, locale) DO NOTHING;

-- verify_email ----------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Verifikasi email Anda',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Verifikasi alamat email Anda</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Terima kasih telah mendaftar di <strong>{{.CompanyName}}</strong>. Satu langkah lagi: konfirmasi bahwa alamat email ini benar milik Anda.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.VerifyURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Verifikasi Email</a></td></tr></table><p style="margin:14px 0 0;font:400 12px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;word-break:break-all;">Tombol tidak berfungsi? Salin tautan berikut ke browser Anda:<br><span style="color:#336699;">{{.VerifyURL}}</span></p><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Tautan berlaku 24 jam. Abaikan email ini jika Anda tidak membuat akun.</p>',
    body_text = 'Halo {{.Name}},

Terima kasih telah mendaftar di {{.CompanyName}}. Verifikasi alamat email Anda melalui tautan berikut:

{{.VerifyURL}}

Tautan berlaku 24 jam. Abaikan email ini jika Anda tidak membuat akun.

-- {{.CompanyName}}'
    WHERE key = 'verify_email' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Verify your email',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Verify your email address</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Thanks for signing up at <strong>{{.CompanyName}}</strong>. One last step: confirm that this email address belongs to you.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.VerifyURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Verify Email</a></td></tr></table><p style="margin:14px 0 0;font:400 12px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;word-break:break-all;">Button not working? Copy this link into your browser:<br><span style="color:#336699;">{{.VerifyURL}}</span></p><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">The link is valid for 24 hours. Ignore this email if you did not create an account.</p>',
    body_text = 'Hello {{.Name}},

Thanks for signing up at {{.CompanyName}}. Verify your email address using this link:

{{.VerifyURL}}

The link is valid for 24 hours. Ignore this email if you did not create an account.

-- {{.CompanyName}}'
    WHERE key = 'verify_email' AND locale = 'en' AND updated_at = created_at;

-- reset_password --------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Atur ulang kata sandi',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Atur ulang kata sandi Anda</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Kami menerima permintaan untuk mengatur ulang kata sandi akun <strong>{{.CompanyName}}</strong> Anda.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.ResetURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Atur Ulang Kata Sandi</a></td></tr></table><p style="margin:14px 0 0;font:400 12px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;word-break:break-all;">Tombol tidak berfungsi? Salin tautan berikut ke browser Anda:<br><span style="color:#336699;">{{.ResetURL}}</span></p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf7ec;border-left:4px solid #f89406;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#6b4e12;">Jika Anda tidak meminta pengaturan ulang, abaikan email ini. Kata sandi Anda tidak akan berubah.</td></tr></table>',
    body_text = 'Halo {{.Name}},

Kami menerima permintaan untuk mengatur ulang kata sandi akun Anda. Gunakan tautan berikut:

{{.ResetURL}}

Jika Anda tidak meminta pengaturan ulang, abaikan email ini. Kata sandi Anda tidak akan berubah.

-- {{.CompanyName}}'
    WHERE key = 'reset_password' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Reset your password',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Reset your password</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">We received a request to reset the password for your <strong>{{.CompanyName}}</strong> account.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.ResetURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Reset Password</a></td></tr></table><p style="margin:14px 0 0;font:400 12px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;word-break:break-all;">Button not working? Copy this link into your browser:<br><span style="color:#336699;">{{.ResetURL}}</span></p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf7ec;border-left:4px solid #f89406;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#6b4e12;">If you did not request a reset, ignore this email. Your password will stay unchanged.</td></tr></table>',
    body_text = 'Hello {{.Name}},

We received a request to reset your account password. Use this link:

{{.ResetURL}}

If you did not request a reset, ignore this email. Your password will stay unchanged.

-- {{.CompanyName}}'
    WHERE key = 'reset_password' AND locale = 'en' AND updated_at = created_at;

-- invoice_created -------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Tagihan baru {{.InvoiceNumber}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Tagihan baru diterbitkan</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Tagihan berikut telah diterbitkan untuk akun Anda.</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Nomor Tagihan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jumlah</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Total}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jatuh Tempo</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.DueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.InvoiceURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Lihat &amp; Bayar Tagihan</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Rincian item tagihan tersedia pada halaman tagihan di portal klien.</p>',
    body_text = 'Halo {{.Name}},

Tagihan baru telah diterbitkan untuk akun Anda.

Nomor Tagihan : {{.InvoiceNumber}}
Jumlah        : {{.Total}}
Jatuh Tempo   : {{.DueDate}}

Bayar di: {{.InvoiceURL}}

-- {{.CompanyName}}'
    WHERE key = 'invoice_created' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'New invoice {{.InvoiceNumber}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">A new invoice has been issued</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">The following invoice has been issued for your account.</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Invoice Number</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Amount</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Total}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Due Date</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.DueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.InvoiceURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">View &amp; Pay Invoice</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">A full line-item breakdown is available on the invoice page in the client portal.</p>',
    body_text = 'Hello {{.Name}},

A new invoice has been issued for your account.

Invoice Number : {{.InvoiceNumber}}
Amount         : {{.Total}}
Due Date       : {{.DueDate}}

Pay at: {{.InvoiceURL}}

-- {{.CompanyName}}'
    WHERE key = 'invoice_created' AND locale = 'en' AND updated_at = created_at;

-- invoice_reminder ------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Pengingat: tagihan {{.InvoiceNumber}} jatuh tempo {{.DueDate}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Pengingat pembayaran</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf7ec;border-left:4px solid #f89406;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#6b4e12;">Tagihan <strong>{{.InvoiceNumber}}</strong> akan jatuh tempo pada <strong>{{.DueDate}}</strong>.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Nomor Tagihan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jumlah</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Total}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jatuh Tempo</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.DueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.InvoiceURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Bayar Sekarang</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Abaikan email ini jika pembayaran Anda sudah dalam proses.</p>',
    body_text = 'Halo {{.Name}},

Pengingat: tagihan {{.InvoiceNumber}} sebesar {{.Total}} akan jatuh tempo pada {{.DueDate}}.

Bayar di: {{.InvoiceURL}}

Abaikan email ini jika pembayaran Anda sudah dalam proses.

-- {{.CompanyName}}'
    WHERE key = 'invoice_reminder' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Reminder: invoice {{.InvoiceNumber}} due {{.DueDate}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Payment reminder</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf7ec;border-left:4px solid #f89406;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#6b4e12;">Invoice <strong>{{.InvoiceNumber}}</strong> is due on <strong>{{.DueDate}}</strong>.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Invoice Number</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Amount</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Total}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Due Date</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.DueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.InvoiceURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Pay Now</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Please ignore this reminder if your payment is already on its way.</p>',
    body_text = 'Hello {{.Name}},

Reminder: invoice {{.InvoiceNumber}} for {{.Total}} is due on {{.DueDate}}.

Pay at: {{.InvoiceURL}}

Please ignore this reminder if your payment is already on its way.

-- {{.CompanyName}}'
    WHERE key = 'invoice_reminder' AND locale = 'en' AND updated_at = created_at;

-- invoice_overdue -------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Tagihan {{.InvoiceNumber}} telah lewat jatuh tempo',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Tagihan lewat jatuh tempo</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;">Tagihan <strong>{{.InvoiceNumber}}</strong> telah melewati jatuh tempo ({{.DueDate}}). Layanan yang terkait berisiko ditangguhkan.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Nomor Tagihan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jumlah</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Total}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jatuh Tempo</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.DueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.InvoiceURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Bayar Sekarang</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Sudah membayar? Konfirmasi pembayaran Anda melalui tiket dukungan.</p>',
    body_text = 'Halo {{.Name}},

Tagihan {{.InvoiceNumber}} sebesar {{.Total}} telah melewati jatuh tempo ({{.DueDate}}). Layanan yang terkait berisiko ditangguhkan.

Bayar di: {{.InvoiceURL}}

-- {{.CompanyName}}'
    WHERE key = 'invoice_overdue' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Invoice {{.InvoiceNumber}} is overdue',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Invoice overdue</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;">Invoice <strong>{{.InvoiceNumber}}</strong> is past its due date ({{.DueDate}}). The related services are at risk of suspension.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Invoice Number</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Amount</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Total}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Due Date</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.DueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.InvoiceURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Pay Now</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Already paid? Let us know through a support ticket.</p>',
    body_text = 'Hello {{.Name}},

Invoice {{.InvoiceNumber}} for {{.Total}} is past its due date ({{.DueDate}}). The related services are at risk of suspension.

Pay at: {{.InvoiceURL}}

-- {{.CompanyName}}'
    WHERE key = 'invoice_overdue' AND locale = 'en' AND updated_at = created_at;

-- payment_received ------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Pembayaran diterima untuk {{.InvoiceNumber}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Pembayaran diterima</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">Pembayaran Anda telah kami terima. Terima kasih!</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Nomor Tagihan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jumlah Dibayar</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Amount}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Tagihan tersebut kini berstatus lunas. Tidak ada tindakan lain yang diperlukan.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/billing/invoices" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Lihat Riwayat Tagihan</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Pembayaran sebesar {{.Amount}} untuk tagihan {{.InvoiceNumber}} telah kami terima. Tagihan tersebut kini berstatus lunas.

Terima kasih!

-- {{.CompanyName}}'
    WHERE key = 'payment_received' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Payment received for {{.InvoiceNumber}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Payment received</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">We have received your payment. Thank you!</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Invoice Number</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.InvoiceNumber}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Amount Paid</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Amount}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">The invoice is now marked as paid. No further action is required.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/billing/invoices" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">View Billing History</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

We received your payment of {{.Amount}} for invoice {{.InvoiceNumber}}. The invoice is now marked as paid.

Thank you!

-- {{.CompanyName}}'
    WHERE key = 'payment_received' AND locale = 'en' AND updated_at = created_at;

-- service_activated -----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan Anda aktif: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Layanan Anda sudah aktif</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Layanan <strong>{{.ServiceName}}</strong> telah selesai disiapkan dan siap digunakan.</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Layanan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr></table><p style="margin:6px 0 0px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Gunakan kredensial berikut untuk masuk ke panel kontrol:</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;background:#f7f9fb;border:1px solid #e1e8ef;border-radius:4px;"><tr><td style="padding:14px 16px;font:400 13px/1.8 Menlo,Consolas,Monaco,monospace;color:#243b53;word-break:break-all;">Username: <strong>{{.Username}}</strong><br>Password: <strong>{{.Password}}</strong></td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.PanelURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Buka Panel Kontrol</a></td></tr></table><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf7ec;border-left:4px solid #f89406;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#6b4e12;">Demi keamanan, segera ganti kata sandi di atas setelah login pertama, dan hapus email ini.</td></tr></table>',
    body_text = 'Halo {{.Name}},

Layanan {{.ServiceName}} ({{.Domain}}) telah aktif.

Username : {{.Username}}
Password : {{.Password}}
Panel    : {{.PanelURL}}

Demi keamanan, segera ganti kata sandi setelah login pertama dan hapus email ini.

-- {{.CompanyName}}'
    WHERE key = 'service_activated' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Your service is active: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Your service is now active</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Your service <strong>{{.ServiceName}}</strong> has finished provisioning and is ready to use.</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Service</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr></table><p style="margin:6px 0 0px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Use the following credentials to sign in to the control panel:</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;background:#f7f9fb;border:1px solid #e1e8ef;border-radius:4px;"><tr><td style="padding:14px 16px;font:400 13px/1.8 Menlo,Consolas,Monaco,monospace;color:#243b53;word-break:break-all;">Username: <strong>{{.Username}}</strong><br>Password: <strong>{{.Password}}</strong></td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.PanelURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Open Control Panel</a></td></tr></table><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf7ec;border-left:4px solid #f89406;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#6b4e12;">For your security, change the password above right after your first login, then delete this email.</td></tr></table>',
    body_text = 'Hello {{.Name}},

Your service {{.ServiceName}} ({{.Domain}}) is now active.

Username : {{.Username}}
Password : {{.Password}}
Panel    : {{.PanelURL}}

For your security, change the password right after your first login and delete this email.

-- {{.CompanyName}}'
    WHERE key = 'service_activated' AND locale = 'en' AND updated_at = created_at;

-- service_suspended -----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan ditangguhkan: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Layanan Anda ditangguhkan</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;">Layanan <strong>{{.ServiceName}}</strong> untuk sementara tidak dapat diakses.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Layanan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Alasan</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Reason}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Lunasi tagihan yang tertunggak untuk mengaktifkan kembali layanan Anda. Data Anda tetap tersimpan selama masa penangguhan.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/billing/invoices" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Lihat Tagihan</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Layanan {{.ServiceName}} ({{.Domain}}) telah ditangguhkan.
Alasan: {{.Reason}}

Lunasi tagihan yang tertunggak untuk mengaktifkan kembali layanan Anda. Data Anda tetap tersimpan selama masa penangguhan.

-- {{.CompanyName}}'
    WHERE key = 'service_suspended' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Service suspended: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Your service has been suspended</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;">Your service <strong>{{.ServiceName}}</strong> is temporarily unreachable.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Service</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Reason</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Reason}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Settle the outstanding invoice to restore the service. Your data is preserved while the service is suspended.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/billing/invoices" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">View Invoices</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

Your service {{.ServiceName}} ({{.Domain}}) has been suspended.
Reason: {{.Reason}}

Settle the outstanding invoice to restore the service. Your data is preserved while the service is suspended.

-- {{.CompanyName}}'
    WHERE key = 'service_suspended' AND locale = 'en' AND updated_at = created_at;

-- service_unsuspended ---------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan diaktifkan kembali: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Layanan Anda aktif kembali</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">Penangguhan telah dicabut. Terima kasih atas pembayaran Anda.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Layanan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Seluruh layanan sudah dapat diakses seperti biasa.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/services" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Lihat Layanan Saya</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Layanan {{.ServiceName}} ({{.Domain}}) telah aktif kembali. Terima kasih atas pembayaran Anda.

-- {{.CompanyName}}'
    WHERE key = 'service_unsuspended' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Service restored: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Your service has been restored</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">The suspension has been lifted. Thank you for your payment.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Service</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Everything is back online and reachable as usual.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/services" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">View My Services</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

Your service {{.ServiceName}} ({{.Domain}}) has been restored. Thank you for your payment.

-- {{.CompanyName}}'
    WHERE key = 'service_unsuspended' AND locale = 'en' AND updated_at = created_at;

-- service_terminated ----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan dihentikan: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Layanan dihentikan</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;">Layanan <strong>{{.ServiceName}}</strong> telah dihentikan secara permanen dan datanya dihapus dari server.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Layanan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Jika ini di luar perkiraan Anda atau Anda ingin berlangganan kembali, silakan hubungi tim kami.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/support" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Hubungi Dukungan</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Layanan {{.ServiceName}} ({{.Domain}}) telah dihentikan secara permanen dan datanya dihapus dari server.

Jika ini di luar perkiraan Anda, silakan hubungi tim kami.

-- {{.CompanyName}}'
    WHERE key = 'service_terminated' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Service terminated: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Service terminated</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;">Your service <strong>{{.ServiceName}}</strong> has been permanently terminated and its data removed from the server.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Service</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">If this was unexpected, or you would like to sign up again, please reach out to us.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/support" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Contact Support</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

Your service {{.ServiceName}} ({{.Domain}}) has been permanently terminated and its data removed from the server.

If this was unexpected, please reach out to us.

-- {{.CompanyName}}'
    WHERE key = 'service_terminated' AND locale = 'en' AND updated_at = created_at;

-- service_renewed -------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Layanan diperpanjang: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Layanan Anda diperpanjang</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">Siklus penagihan layanan Anda berhasil diperpanjang.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Layanan</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Jatuh Tempo Berikutnya</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.NextDueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/services" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Lihat Layanan Saya</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Layanan {{.ServiceName}} ({{.Domain}}) telah diperpanjang.
Jatuh tempo berikutnya: {{.NextDueDate}}

-- {{.CompanyName}}'
    WHERE key = 'service_renewed' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Service renewed: {{.ServiceName}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Your service has been renewed</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">The billing cycle for your service has been renewed.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Service</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ServiceName}}</td></tr><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Next Due Date</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.NextDueDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/services" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">View My Services</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

Your service {{.ServiceName}} ({{.Domain}}) has been renewed.
Next due date: {{.NextDueDate}}

-- {{.CompanyName}}'
    WHERE key = 'service_renewed' AND locale = 'en' AND updated_at = created_at;

-- domain_registered -----------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Domain terdaftar: {{.Domain}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Domain berhasil didaftarkan</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">Domain <strong>{{.Domain}}</strong> kini terdaftar atas nama Anda.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Berlaku Hingga</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ExpiryDate}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Anda dapat mengelola nameserver, kontak, dan pengaturan lainnya di portal klien.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/domains" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Kelola Domain</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Perubahan nameserver dapat memerlukan waktu hingga 24 jam untuk menyebar.</p>',
    body_text = 'Halo {{.Name}},

Domain {{.Domain}} berhasil didaftarkan dan berlaku hingga {{.ExpiryDate}}.

Kelola domain di: {{.FrontendURL}}/domains

-- {{.CompanyName}}'
    WHERE key = 'domain_registered' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Domain registered: {{.Domain}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Domain registered successfully</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">The domain <strong>{{.Domain}}</strong> is now registered to you.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Expires On</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ExpiryDate}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">You can manage nameservers, contacts and other settings in the client portal.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/domains" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Manage Domain</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Nameserver changes can take up to 24 hours to propagate.</p>',
    body_text = 'Hello {{.Name}},

The domain {{.Domain}} has been registered and expires on {{.ExpiryDate}}.

Manage it at: {{.FrontendURL}}/domains

-- {{.CompanyName}}'
    WHERE key = 'domain_registered' AND locale = 'en' AND updated_at = created_at;

-- domain_renewed --------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Domain diperpanjang: {{.Domain}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Domain berhasil diperpanjang</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">Masa berlaku domain <strong>{{.Domain}}</strong> telah diperpanjang.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Berlaku Hingga</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ExpiryDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/domains" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Kelola Domain</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Domain {{.Domain}} telah diperpanjang dan kini berlaku hingga {{.ExpiryDate}}.

-- {{.CompanyName}}'
    WHERE key = 'domain_renewed' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Domain renewed: {{.Domain}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Domain renewed successfully</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f1f8f2;border-left:4px solid #218739;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#1d5c2c;">The registration for <strong>{{.Domain}}</strong> has been extended.</td></tr></table><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Domain</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Domain}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Expires On</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.ExpiryDate}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/domains" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Manage Domain</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

The domain {{.Domain}} has been renewed and now expires on {{.ExpiryDate}}.

-- {{.CompanyName}}'
    WHERE key = 'domain_renewed' AND locale = 'en' AND updated_at = created_at;

-- ticket_opened ---------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Tiket dibuka: {{.TicketNumber}} - {{.Subject}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Tiket dukungan Anda telah dibuat</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Kami telah menerima permintaan Anda. Tim dukungan akan segera meninjaunya.</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Nomor Tiket</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.TicketNumber}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Subjek</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Subject}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.TicketURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Lihat Tiket</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Balas melalui portal klien agar seluruh percakapan tercatat pada tiket ini.</p>',
    body_text = 'Halo {{.Name}},

Tiket {{.TicketNumber}} telah dibuat.
Subjek: {{.Subject}}

Lihat tiket di: {{.TicketURL}}

-- {{.CompanyName}}'
    WHERE key = 'ticket_opened' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'Ticket opened: {{.TicketNumber}} - {{.Subject}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Your support ticket has been created</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">We have received your request. Our support team will review it shortly.</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Ticket Number</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.TicketNumber}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Subject</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Subject}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.TicketURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">View Ticket</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Reply through the client portal so the whole conversation stays on this ticket.</p>',
    body_text = 'Hello {{.Name}},

Ticket {{.TicketNumber}} has been created.
Subject: {{.Subject}}

View it at: {{.TicketURL}}

-- {{.CompanyName}}'
    WHERE key = 'ticket_opened' AND locale = 'en' AND updated_at = created_at;

-- ticket_replied --------------------------------------------------------------
UPDATE email_templates SET
    subject   = 'Balasan baru pada tiket {{.TicketNumber}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Ada balasan baru pada tiket Anda</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Halo {{.Name}},</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Nomor Tiket</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.TicketNumber}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Subjek</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Subject}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Buka tiket untuk membaca balasan selengkapnya dan menanggapinya.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.TicketURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Baca Balasan</a></td></tr></table>',
    body_text = 'Halo {{.Name}},

Ada balasan baru pada tiket {{.TicketNumber}} ({{.Subject}}).

Baca di: {{.TicketURL}}

-- {{.CompanyName}}'
    WHERE key = 'ticket_replied' AND locale = 'id' AND updated_at = created_at;
UPDATE email_templates SET
    subject   = 'New reply on ticket {{.TicketNumber}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">New reply on your ticket</h1><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Hello {{.Name}},</p><table role="presentation" class="em-kv" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0;border:1px solid #e8e8e8;border-radius:4px;"><tr><td style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Ticket Number</td><td class="em-v" style="padding:11px 16px;border-bottom:1px solid #f0f0f0;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.TicketNumber}}</td></tr><tr><td style="padding:11px 16px;font:400 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b7b7b;width:44%;">Subject</td><td class="em-v" style="padding:11px 16px;font:600 13px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">{{.Subject}}</td></tr></table><p style="margin:0px 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">Open the ticket to read the full reply and respond.</p><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.TicketURL}}" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Read Reply</a></td></tr></table>',
    body_text = 'Hello {{.Name}},

There is a new reply on ticket {{.TicketNumber}} ({{.Subject}}).

Read it at: {{.TicketURL}}

-- {{.CompanyName}}'
    WHERE key = 'ticket_replied' AND locale = 'en' AND updated_at = created_at;

-- admin_alert -----------------------------------------------------------------
UPDATE email_templates SET
    subject   = '[{{.CompanyName}}] Peringatan: {{.Subject}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Peringatan sistem</h1><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;"><strong>{{.Subject}}</strong></td></tr></table><p style="margin:6px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Rincian teknis:</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f6f6f6;border:1px solid #e4e4e4;border-radius:4px;"><tr><td style="padding:14px 16px;font:400 12px/1.7 Menlo,Consolas,Monaco,monospace;color:#333333;white-space:pre-wrap;word-break:break-word;">{{.Detail}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/admin/logs" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Buka Log Admin</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Pesan ini dikirim ke alamat peringatan operator (ADMIN_ALERT_EMAIL).</p>',
    body_text = 'Peringatan sistem {{.CompanyName}}

{{.Subject}}

Rincian:
{{.Detail}}
'
    WHERE key = 'admin_alert' AND locale = 'id' AND (updated_at = created_at OR body_html = '<p>Peringatan sistem WHCMS:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>');
UPDATE email_templates SET
    subject   = '[{{.CompanyName}}] Alert: {{.Subject}}',
    body_html = '<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">System alert</h1><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#fdf1f0;border-left:4px solid #c43c35;border-radius:3px;"><tr><td style="padding:13px 16px;font:400 14px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#7b2622;"><strong>{{.Subject}}</strong></td></tr></table><p style="margin:6px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">Technical detail:</p><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:20px 0;background:#f6f6f6;border:1px solid #e4e4e4;border-radius:4px;"><tr><td style="padding:14px 16px;font:400 12px/1.7 Menlo,Consolas,Monaco,monospace;color:#333333;white-space:pre-wrap;word-break:break-word;">{{.Detail}}</td></tr></table><table role="presentation" class="em-btn" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 6px;"><tr><td align="center" style="border-radius:4px;background:#336699;"><a href="{{.FrontendURL}}/admin/logs" style="display:inline-block;padding:13px 30px;font:600 15px/1 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#ffffff;text-decoration:none;border-radius:4px;">Open Admin Logs</a></td></tr></table><p style="margin:18px 0 0px;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">This message was sent to the operator alert address (ADMIN_ALERT_EMAIL).</p>',
    body_text = '{{.CompanyName}} system alert

{{.Subject}}

Detail:
{{.Detail}}
'
    WHERE key = 'admin_alert' AND locale = 'en' AND (updated_at = created_at OR body_html = '<p>WHCMS system alert:</p><p><strong>{{.Subject}}</strong></p><pre>{{.Detail}}</pre>');
