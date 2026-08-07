-- Real WHM reseller accounts commonly require every package name to carry
-- the reseller's own prefix (e.g. "reseller_whcms_s43") — addpkg/createacct
-- against a bare "whcms_s43" fails there even though it works fine on a
-- dedicated/non-reseller server. Optional, blank by default (preserves
-- current behavior for servers that don't need it).
ALTER TABLE servers ADD COLUMN package_prefix TEXT NOT NULL DEFAULT '';
