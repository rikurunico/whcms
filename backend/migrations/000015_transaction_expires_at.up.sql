-- Persists the pending-payment expiry that payments.Service.payWithGateway
-- already computes at transaction creation, so a page reload can resume the
-- same VA/QRIS/bank-transfer instructions instead of silently creating a new
-- gateway transaction on every visit.
ALTER TABLE transactions ADD COLUMN expires_at TIMESTAMPTZ NULL;
