alter table field_definition
	add constraint field_definition_object_field_unique
	unique (tenant_bucket, tenant_id, metadata_version_id, object_id, field_id);

alter table object_record
	add column metadata_version_id uuid,
	add column owner_id text,
	add column lifecycle_state varchar(20) not null default 'active'
		check (lifecycle_state in ('active', 'deleted')),
	add column created_by text not null default 'migration',
	add column updated_by text not null default 'migration',
	add column deleted_at timestamptz,
	add constraint object_record_metadata_object_fk
		foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id)
		references object_definition(tenant_bucket, tenant_id, metadata_version_id, object_id),
	add constraint object_record_metadata_record_unique
		unique (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id),
	add constraint object_record_delete_state_check check (
		(lifecycle_state = 'active' and deleted_at is null)
		or (lifecycle_state = 'deleted' and deleted_at is not null)
	);

create index object_record_active_updated_idx
	on object_record(tenant_bucket, tenant_id, object_id, updated_at desc, record_id)
	where lifecycle_state = 'active';

alter table record_relation
	add column metadata_version_id uuid,
	add constraint record_relation_metadata_definition_fk
		foreign key (tenant_bucket, tenant_id, metadata_version_id, relation_definition_id)
		references relation_definition(tenant_bucket, tenant_id, metadata_version_id, relation_id),
	add constraint record_relation_metadata_source_fk
		foreign key (tenant_bucket, tenant_id, metadata_version_id, source_object_id, source_record_id)
		references object_record(tenant_bucket, tenant_id, metadata_version_id, object_id, record_id);

create index record_relation_definition_source_idx
	on record_relation(tenant_bucket, tenant_id, metadata_version_id, relation_definition_id, source_object_id, source_record_id);

create table record_operation (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	capability_id text not null,
	idempotency_key text not null,
	request_hash char(64) not null,
	status varchar(16) not null check (status in ('running', 'succeeded')),
	result jsonb,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, capability_id, idempotency_key),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id),
	check (
		(status = 'running' and result is null)
		or (status = 'succeeded' and result is not null)
	)
);

create table record_index_text (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	record_id uuid not null,
	value_text text collate "C" not null,
	primary key (tenant_bucket, tenant_id, object_id, field_id, record_id),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id)
		references object_record(tenant_bucket, tenant_id, metadata_version_id, object_id, record_id) on delete cascade,
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
		references field_definition(tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
) partition by list (tenant_bucket);

create table record_index_number (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	record_id uuid not null,
	value_number numeric(38, 12) not null,
	primary key (tenant_bucket, tenant_id, object_id, field_id, record_id),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id)
		references object_record(tenant_bucket, tenant_id, metadata_version_id, object_id, record_id) on delete cascade,
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
		references field_definition(tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
) partition by list (tenant_bucket);

create table record_index_boolean (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	record_id uuid not null,
	value_boolean boolean not null,
	primary key (tenant_bucket, tenant_id, object_id, field_id, record_id),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id)
		references object_record(tenant_bucket, tenant_id, metadata_version_id, object_id, record_id) on delete cascade,
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
		references field_definition(tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
) partition by list (tenant_bucket);

create table record_index_datetime (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	record_id uuid not null,
	value_kind varchar(8) not null check (value_kind in ('date', 'datetime')),
	value_datetime timestamptz not null,
	primary key (tenant_bucket, tenant_id, object_id, field_id, record_id),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id)
		references object_record(tenant_bucket, tenant_id, metadata_version_id, object_id, record_id) on delete cascade,
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
		references field_definition(tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
) partition by list (tenant_bucket);

create table record_index_uuid (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	metadata_version_id uuid not null,
	object_id uuid not null,
	field_id uuid not null,
	record_id uuid not null,
	value_uuid uuid not null,
	primary key (tenant_bucket, tenant_id, object_id, field_id, record_id),
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, record_id)
		references object_record(tenant_bucket, tenant_id, metadata_version_id, object_id, record_id) on delete cascade,
	foreign key (tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
		references field_definition(tenant_bucket, tenant_id, metadata_version_id, object_id, field_id)
) partition by list (tenant_bucket);

do $$
declare
	bucket integer;
	base_name text;
	partition_name text;
	value_column text;
begin
	foreach base_name in array array['record_index_text','record_index_number','record_index_boolean','record_index_datetime','record_index_uuid'] loop
		case base_name
			when 'record_index_text' then value_column := 'value_text';
			when 'record_index_number' then value_column := 'value_number';
			when 'record_index_boolean' then value_column := 'value_boolean';
			when 'record_index_datetime' then value_column := 'value_datetime';
			when 'record_index_uuid' then value_column := 'value_uuid';
		end case;
		for bucket in 0..127 loop
			partition_name := format('%s_b%s', base_name, lpad(bucket::text, 3, '0'));
			execute format('create table %I partition of %I for values in (%s)', partition_name, base_name, bucket);
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
			execute format(
				'create index %I on %I (tenant_id, object_id, field_id, %I, record_id)',
				partition_name || '_value_idx', partition_name, value_column
			);
		end loop;
	end loop;
end
$$;

alter table record_index_text enable row level security;
alter table record_index_text force row level security;
alter table record_index_number enable row level security;
alter table record_index_number force row level security;
alter table record_index_boolean enable row level security;
alter table record_index_boolean force row level security;
alter table record_index_datetime enable row level security;
alter table record_index_datetime force row level security;
alter table record_index_uuid enable row level security;
alter table record_index_uuid force row level security;
alter table record_operation enable row level security;
alter table record_operation force row level security;

create policy tenant_isolation on record_index_text using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_index_number using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_index_boolean using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_index_datetime using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_index_uuid using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_operation using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

grant select, insert, update, delete on
	record_index_text,
	record_index_number,
	record_index_boolean,
	record_index_datetime,
	record_index_uuid,
	record_operation
to ai_native_runtime;
