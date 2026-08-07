ALTER TABLE domains
    DROP COLUMN dns_management_enabled,
    DROP COLUMN email_forwarding_enabled;

DROP TABLE domain_addons;
DROP TABLE premium_domain_pricing;
DROP TABLE tld_pricing;
