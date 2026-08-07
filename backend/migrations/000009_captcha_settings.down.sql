DELETE FROM settings WHERE key IN (
    'security.captcha_enabled',
    'security.captcha_provider',
    'security.captcha_site_key'
);
