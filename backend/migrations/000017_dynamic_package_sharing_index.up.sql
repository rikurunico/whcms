-- Supports provisioning.CountByServerAndPackage: deciding whether a shared
-- dynamic-product control-panel package can be deleted on service termination.
CREATE INDEX IF NOT EXISTS services_server_package_name_idx
    ON services (server_id, (panel_meta->>'package_name'))
    WHERE panel_meta ? 'package_name';
