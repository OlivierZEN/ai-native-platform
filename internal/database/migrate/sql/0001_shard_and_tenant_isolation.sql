do $$
begin
	if not exists (select 1 from pg_roles where rolname = 'ai_native_runtime') then
		create role ai_native_runtime nologin nosuperuser nocreatedb nocreaterole noinherit nobypassrls;
	end if;
end
$$;

create table shard_registry (
	shard_id varchar(32) primary key,
	region varchar(32) not null,
	status varchar(24) not null check (status in ('active', 'draining', 'offline')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp()
);

insert into shard_registry(shard_id, region, status)
values ('shard-001', 'cn-default', 'active')
on conflict (shard_id) do nothing;

create table tenant_registry (
	tenant_id uuid primary key,
	org_id varchar(20) collate "C" not null unique check (org_id ~ '^org[a-z0-9]{17}$'),
	display_name varchar(200) not null,
	shard_id varchar(32) not null references shard_registry(shard_id),
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	service_tier varchar(24) not null,
	global_lifecycle_status varchar(24) not null check (global_lifecycle_status in ('active', 'suspended', 'decommissioned')),
	native_status varchar(24) not null check (native_status in ('provisioning', 'active', 'suspended', 'failed', 'decommissioned')),
	tenant_revision bigint not null check (tenant_revision > 0),
	product_revision bigint not null check (product_revision > 0),
	route_revision bigint not null check (route_revision > 0),
	entitlements jsonb not null default '{}'::jsonb,
	metadata_version_id uuid,
	last_operation_id text,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	unique (tenant_bucket, tenant_id),
	unique (tenant_id, org_id)
);

create table tenant_operation (
	operation_id text primary key,
	tenant_bucket smallint not null,
	tenant_id uuid not null,
	capability_id text not null,
	request_hash char(64) not null,
	status varchar(24) not null check (status in ('running', 'succeeded', 'failed', 'pending_approval')),
	result jsonb,
	error_code text,
	product_revision bigint not null check (product_revision > 0),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
);

create table audit_event (
	audit_id uuid primary key,
	request_id text not null,
	operation_id text,
	tenant_bucket smallint not null,
	tenant_id uuid not null,
	actor_id text not null,
	capability_id text not null,
	status varchar(24) not null,
	error_code text,
	event_data jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default clock_timestamp(),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
);

create table object_record (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid not null,
	record_id uuid not null,
	data jsonb not null default '{}'::jsonb,
	revision bigint not null default 1 check (revision > 0),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, object_id, record_id),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
) partition by list (tenant_bucket);

do $$
declare
	bucket integer;
	partition_name text;
begin
	for bucket in 0..127 loop
		partition_name := format('object_record_b%s', lpad(bucket::text, 3, '0'));
		execute format('create table %I partition of object_record for values in (%s)', partition_name, bucket);
		execute format('alter table %I enable row level security', partition_name);
		execute format('alter table %I force row level security', partition_name);
		execute format(
			'create policy tenant_isolation on %I using (
				tenant_id = nullif(current_setting(''app.tenant_id'', true), '''')::uuid
				and tenant_bucket = nullif(current_setting(''app.tenant_bucket'', true), '''')::smallint
			) with check (
				tenant_id = nullif(current_setting(''app.tenant_id'', true), '''')::uuid
				and tenant_bucket = nullif(current_setting(''app.tenant_bucket'', true), '''')::smallint
			)',
			partition_name
		);
	end loop;
end
$$;

create table record_relation (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	relation_id uuid not null,
	relation_definition_id uuid not null,
	source_object_id uuid not null,
	source_record_id uuid not null,
	target_object_id uuid not null,
	target_record_id uuid not null,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, relation_id),
	foreign key (tenant_bucket, tenant_id, source_object_id, source_record_id)
		references object_record(tenant_bucket, tenant_id, object_id, record_id),
	foreign key (tenant_bucket, tenant_id, target_object_id, target_record_id)
		references object_record(tenant_bucket, tenant_id, object_id, record_id)
);

create index tenant_operation_tenant_updated_idx on tenant_operation(tenant_bucket, tenant_id, updated_at desc);
create index audit_event_tenant_created_idx on audit_event(tenant_bucket, tenant_id, created_at desc);
create index record_relation_source_idx on record_relation(tenant_bucket, tenant_id, source_object_id, source_record_id);
create index record_relation_target_idx on record_relation(tenant_bucket, tenant_id, target_object_id, target_record_id);

alter table tenant_registry enable row level security;
alter table tenant_registry force row level security;
alter table tenant_operation enable row level security;
alter table tenant_operation force row level security;
alter table audit_event enable row level security;
alter table audit_event force row level security;
alter table object_record enable row level security;
alter table object_record force row level security;
alter table record_relation enable row level security;
alter table record_relation force row level security;

create policy tenant_isolation on tenant_registry using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on tenant_operation using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on audit_event using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on object_record using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_relation using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

grant usage on schema public to ai_native_runtime;
grant select on tenant_registry to ai_native_runtime;
grant select, insert, update, delete on tenant_operation, audit_event, object_record, record_relation to ai_native_runtime;
