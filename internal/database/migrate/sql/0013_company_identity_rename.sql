-- The global operations identity is a company identifier. Keep the established
-- values intact, but make the database contract unambiguous with respect to
-- the tenant-internal authorization organization tree.
alter table tenant_registry rename column org_id to company_id;

alter table tenant_registry rename constraint tenant_registry_org_id_key to tenant_registry_company_id_key;
alter table tenant_registry rename constraint tenant_registry_org_id_check to tenant_registry_company_id_check;
alter table tenant_registry rename constraint tenant_registry_tenant_id_org_id_key to tenant_registry_tenant_id_company_id_key;
