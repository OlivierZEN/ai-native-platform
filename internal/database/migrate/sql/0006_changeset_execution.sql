alter table field_definition
	add column unique_value boolean not null default false;

alter table field_definition
	add constraint field_definition_unique_contract_check check (
		not unique_value or (indexed and data_type <> 'json' and lifecycle_state <> 'tombstone')
	);

alter table metadata_changeset_object
	add column skipped_conflicts bigint not null default 0 check (skipped_conflicts >= 0),
	add column retry_records bigint not null default 0 check (retry_records >= 0),
	add column failed_samples jsonb not null default '[]'::jsonb,
	add column completed_at timestamptz;

create table governance_policy (
	service_tier varchar(24) not null,
	policy_version bigint not null check (policy_version > 0),
	max_fields_per_object integer not null check (max_fields_per_object between 1 and 500),
	max_active_indexed_fields integer not null check (max_active_indexed_fields between 1 and 50),
	max_record_json_bytes integer not null check (max_record_json_bytes between 1024 and 262144),
	max_json_field_bytes integer not null check (max_json_field_bytes between 1024 and 65536),
	max_json_depth integer not null check (max_json_depth between 1 and 8),
	max_json_array_elements integer not null check (max_json_array_elements between 1 and 1000),
	active boolean not null default true,
	created_at timestamptz not null default clock_timestamp(),
	primary key (service_tier, policy_version)
);

create unique index governance_policy_one_active_idx
	on governance_policy(service_tier) where active;

insert into governance_policy(
	service_tier,policy_version,max_fields_per_object,max_active_indexed_fields,
	max_record_json_bytes,max_json_field_bytes,max_json_depth,max_json_array_elements
) values
	('standard',1,500,20,262144,65536,8,1000),
	('dedicated-16g',1,500,40,262144,65536,8,1000);

create table record_unique_value (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	record_id uuid not null,
	value_hash bytea not null check (octet_length(value_hash) = 32),
	value_canonical text not null check (octet_length(value_canonical) <= 1024),
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,object_id,field_id,value_hash),
	unique (tenant_bucket,tenant_id,object_id,field_id,record_id),
	foreign key (tenant_bucket,tenant_id,metadata_version_id,object_id,record_id)
		references object_record(tenant_bucket,tenant_id,metadata_version_id,object_id,record_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,metadata_version_id,object_id,field_id)
		references field_definition(tenant_bucket,tenant_id,metadata_version_id,object_id,field_id)
);

create index record_unique_value_record_idx
	on record_unique_value(tenant_bucket,tenant_id,object_id,record_id);

alter table record_unique_value enable row level security;
alter table record_unique_value force row level security;

create policy tenant_isolation on record_unique_value using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

create table field_tombstone (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	api_name varchar(96) collate "C" not null,
	metadata_version_id uuid not null,
	tombstoned_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,object_id,field_id),
	unique (tenant_bucket,tenant_id,object_id,api_name),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

alter table field_tombstone enable row level security;
alter table field_tombstone force row level security;

create policy tenant_isolation on field_tombstone using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

grant select on governance_policy to ai_native_runtime;
grant select, insert, update, delete on record_unique_value, field_tombstone to ai_native_runtime;
