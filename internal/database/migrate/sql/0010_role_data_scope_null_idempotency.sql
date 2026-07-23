-- PostgreSQL unique constraints treat NULL values as distinct. A role's
-- unanchored scopes (own, assigned_organizations, all_tenant) must still be
-- idempotent under retried approved capability calls.
create unique index role_data_scope_unanchored_unique_idx
	on role_data_scope(tenant_bucket,tenant_id,role_id,object_id,action,scope_type)
	where organization_id is null;
