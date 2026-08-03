-- FEAT-035: display and governance facts for AgentCiCi-authoritative HUMAN/SERVICE principals.
alter table principal_projection
	add column display_name varchar(200),
	add column public_id varchar(64),
	add column owner_principal_id text,
	add column client_id varchar(255),
	add column authority_source varchar(32) not null default 'agentcici',
	add column last_synced_at timestamptz;

alter table principal_projection
	add constraint principal_projection_authority_source_check
	check (authority_source in ('agentcici','legacy')),
	add constraint principal_projection_owner_fk
	foreign key (tenant_bucket,tenant_id,owner_principal_id)
		references principal_projection(tenant_bucket,tenant_id,principal_id)
		deferrable initially deferred;

create unique index principal_projection_client_id_idx
	on principal_projection(tenant_bucket,tenant_id,client_id)
	where client_id is not null;

create index principal_projection_owner_idx
	on principal_projection(tenant_bucket,tenant_id,owner_principal_id)
	where owner_principal_id is not null;
