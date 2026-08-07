DROP INDEX IF EXISTS idx_email_log_user_id;
ALTER TABLE email_log DROP COLUMN IF EXISTS user_id;
