create table metadata_version (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	sequence bigint not null check (sequence > 0),
	status varchar(16) not null check (status in ('draft', 'published', 'retired')),
	snapshot jsonb,
	snapshot_digest char(64),
	created_by text not null,
	published_by text,
	created_at timestamptz not null default clock_timestamp(),
	published_at timestamptz,
	primary key (tenant_bucket, tenant_id, metadata_version_id),
	unique (tenant_bucket, tenant_id, sequence),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id),
	check (
		(status = 'draft' and snapshot is null and snapshot_digest is null and published_by is null and published_at is null)
		or
		(status in ('published', 'retired') and snapshot is not null and snapshot_digest is not null and published_by is not null and published_at is not null)
	)
);

create table object_definition (
	tenant_bucket smallint not null,
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	api_name varchar(96) collate "C" not null check (api_name ~ '^[a-z][a-z0-9_]{0,95}$'),
	label varchar(200) not null,
	description text not null default '',
	semantic jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, metadata_version_id, object_id),
	unique (tenant_bucket, tenant_id, metadata_version_id, api_name),
	foreign key (tenant_bucket, tenant_id, metadata_version_id)
		references metadata_version(tenant_bucket, tenant_id, metadata_version_id) on delete cascade
);

create table field_definition (
	tenant_bucket smallint not null,
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	field_id uuid not null,
	object_id uuid not null,
	api_name varchar(96) collate "C" not null check (api_name ~ '^[a-z][a-z0-9_]{0,95}$'),
	label varchar(200) not null,
	description text not null default '',
	data_type varchar(24) not null check (data_type in ('text', 'number', 'boolean', 'date', 'datetime', 'uuid', 'json')),
	required boolean not null default false,
	indexed boolean not null default false,
	default_value jsonb,
	constraints jsonb not null default '{}'::jsonb,
	semantic jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, metadata_version_id, field_id),
	unique (tenant_bucket, tenant_id, metadata_version_id, object_id, api_name),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id)
		references object_definition(tenant_bucket, tenant_id, metadata_version_id, object_id) on delete cascade
);

create table relation_definition (
	tenant_bucket smallint not null,
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	relation_id uuid not null,
	api_name varchar(96) collate "C" not null check (api_name ~ '^[a-z][a-z0-9_]{0,95}$'),
	source_object_id uuid not null,
	target_object_id uuid not null,
	relation_type varchar(24) not null check (relation_type in ('lookup', 'master_detail', 'many_to_many')),
	delete_behavior varchar(24) not null check (delete_behavior in ('restrict', 'cascade', 'set_null')),
	description text not null default '',
	semantic jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, metadata_version_id, relation_id),
	unique (tenant_bucket, tenant_id, metadata_version_id, api_name),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, source_object_id)
		references object_definition(tenant_bucket, tenant_id, metadata_version_id, object_id),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, target_object_id)
		references object_definition(tenant_bucket, tenant_id, metadata_version_id, object_id)
);

create or replace function reject_non_draft_metadata_definition()
returns trigger
language plpgsql
as $$
declare
	version_status varchar(16);
	target_bucket smallint;
	target_tenant uuid;
	target_version uuid;
begin
	target_bucket := coalesce(new.tenant_bucket, old.tenant_bucket);
	target_tenant := coalesce(new.tenant_id, old.tenant_id);
	target_version := coalesce(new.metadata_version_id, old.metadata_version_id);
	select status into version_status
	from metadata_version
	where tenant_bucket = target_bucket and tenant_id = target_tenant and metadata_version_id = target_version;
	if version_status is distinct from 'draft' then
		raise exception 'metadata definitions are immutable outside a draft version' using errcode = '55000';
	end if;
	if tg_op = 'DELETE' then
		return old;
	end if;
	return new;
end
$$;

create or replace function reject_published_metadata_version_mutation()
returns trigger
language plpgsql
as $$
begin
	if old.status in ('published', 'retired') then
		raise exception 'published metadata versions are immutable' using errcode = '55000';
	end if;
	if tg_op = 'DELETE' then
		return old;
	end if;
	return new;
end
$$;

create trigger object_definition_draft_only
before insert or update or delete on object_definition
for each row execute function reject_non_draft_metadata_definition();
create trigger field_definition_draft_only
before insert or update or delete on field_definition
for each row execute function reject_non_draft_metadata_definition();
create trigger relation_definition_draft_only
before insert or update or delete on relation_definition
for each row execute function reject_non_draft_metadata_definition();
create trigger metadata_version_immutable
before update or delete on metadata_version
for each row execute function reject_published_metadata_version_mutation();

alter table metadata_version enable row level security;
alter table metadata_version force row level security;
alter table object_definition enable row level security;
alter table object_definition force row level security;
alter table field_definition enable row level security;
alter table field_definition force row level security;
alter table relation_definition enable row level security;
alter table relation_definition force row level security;

create policy tenant_isolation on metadata_version using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on object_definition using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on field_definition using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on relation_definition using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

grant select, insert, update, delete on metadata_version, object_definition, field_definition, relation_definition to ai_native_runtime;
grant update(metadata_version_id, updated_at) on tenant_registry to ai_native_runtime;
