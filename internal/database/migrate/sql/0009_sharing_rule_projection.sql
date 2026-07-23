alter table sharing_rule_def
	add column projection_state varchar(16) not null default 'building' check (projection_state in ('building','ready','failed')),
	add column projection_cursor uuid,
	add column projection_revision bigint not null default 1 check (projection_revision > 0),
	add column projection_error text;

create index sharing_rule_projection_refresh_idx
	on sharing_rule_def(tenant_bucket,tenant_id,projection_state,rule_id);
