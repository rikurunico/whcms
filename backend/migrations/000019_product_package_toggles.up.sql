-- Long-tail per-package toggles for configurable (dynamic-spec) products,
-- only read when products.configurable is true (see buildPackageSpec).
ALTER TABLE products ADD COLUMN shell_access BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE products ADD COLUMN cgi_access BOOLEAN NOT NULL DEFAULT FALSE;
-- cPanel only: name of an existing WHM Feature List the admin manages
-- directly in WHM (live reference — addpkg/editpkg featurelist param).
ALTER TABLE products ADD COLUMN feature_list TEXT NOT NULL DEFAULT '';
-- DirectAdmin only: existing DA package whose long-tail fields (Git,
-- WordPress, ClamAV, etc. — anything not modeled by our own ProvisionKey
-- knobs) are read and merged into the per-service package, since
-- DirectAdmin has no separate feature-list object like cPanel's.
ALTER TABLE products ADD COLUMN template_package TEXT NOT NULL DEFAULT '';
