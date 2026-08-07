-- Links each outbound email to the account it was sent to, so a client (or
-- staff member) can view their own delivery history and an admin can filter
-- by user, not just by recipient address (SendTemplate always knows the
-- recipient's user id; AlertAdmin/SendTestEmail leave it NULL since they
-- aren't addressed to a specific account holder).
ALTER TABLE email_log ADD COLUMN user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX idx_email_log_user_id ON email_log(user_id);
