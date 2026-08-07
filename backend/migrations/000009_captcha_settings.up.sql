-- Optional Cloudflare Turnstile CAPTCHA on spam-prone public actions
-- (login, register, order checkout). Production-safe defaults: disabled, no
-- site key configured. Operators enable it and set their public site key via
-- Admin → Settings → Security; the secret key stays ENV-only
-- (TURNSTILE_SECRET_KEY). See internal/service/captcha.
INSERT INTO settings (key, value) VALUES
    ('security.captcha_enabled',  'false'),
    ('security.captcha_provider', '"turnstile"'),
    ('security.captcha_site_key', '""')
ON CONFLICT (key) DO NOTHING;
