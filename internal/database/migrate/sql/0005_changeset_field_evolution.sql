alter table field_definition
	add column lifecycle_state varchar(32) not null default 'active',
	add column index_state varchar(16) not null default 'active',
	add column default_semantics varchar(24) not null default 'on_create',
	add column predecessor_field_id uuid;

alter table field_definition disable trigger field_definition_draft_only;
update field_definition set index_state = 'none' where not indexed;
alter table field_definition enable trigger field_definition_draft_only;

alter table field_definition
	add constraint field_definition_lifecycle_state_check check (
		lifecycle_state in ('active','deprecated_read_write','deprecated_read_only','hidden','purging','tombstone')
	),
	add constraint field_definition_index_state_check check (
		index_state in ('none','building','validating','active','failed','retiring')
	),
	add constraint field_definition_default_semantics_check check (
		default_semantics in ('on_create','backfill_required')
	),
	add constraint field_definition_index_contract_check check (
		(indexed and index_state <> 'none') or (not indexed and index_state = 'none')
	),
	add constraint field_definition_tombstone_contract_check check (
		lifecycle_state <> 'tombstone' or (not required and not indexed and index_state = 'none')
	);

create table metadata_changeset (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	changeset_id uuid not null,
	base_metadata_version_id uuid,
	candidate_metadata_version_id uuid not null,
	state varchar(24) not null check (
		state in ('validated','approved','backfilling','ready','active','failed','canceled','rolled_back')
	),
	risk_level varchar(16) not null check (risk_level in ('low','medium','high')),
	requires_backfill boolean not null,
	operation_digest char(64) not null,
	quota_snapshot jsonb not null,
	plan jsonb not null,
	simulation jsonb not null,
	coverage jsonb not null default '{}'::jsonb,
	approval_id text,
	approved_by text,
	created_by text not null,
	last_error_code text,
	last_error_message text,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	approved_at timestamptz,
	activated_at timestamptz,
	primary key (tenant_bucket, tenant_id, changeset_id),
	unique (tenant_bucket, tenant_id, candidate_metadata_version_id),
	unique (tenant_bucket, tenant_id, changeset_id, candidate_metadata_version_id),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id),
	foreign key (tenant_bucket, tenant_id, base_metadata_version_id)
		references metadata_version(tenant_bucket, tenant_id, metadata_version_id),
	foreign key (tenant_bucket, tenant_id, candidate_metadata_version_id)
		references metadata_version(tenant_bucket, tenant_id, metadata_version_id),
	check (
		(state in ('approved','backfilling','ready','active','rolled_back') and approval_id is not null and approved_by is not null and approved_at is not null)
		or state in ('validated','failed','canceled')
	)
);

create table metadata_changeset_object (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	changeset_id uuid not null,
	candidate_metadata_version_id uuid not null,
	object_id uuid not null,
	state varchar(16) not null check (state in ('pending','backfilling','ready','failed')),
	cursor_record_id uuid,
	eligible_records bigint not null default 0 check (eligible_records >= 0),
	processed_records bigint not null default 0 check (processed_records >= 0),
	succeeded_records bigint not null default 0 check (succeeded_records >= 0),
	failed_records bigint not null default 0 check (failed_records >= 0),
	defaults jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, changeset_id, object_id),
	foreign key (tenant_bucket, tenant_id, changeset_id, candidate_metadata_version_id)
		references metadata_changeset(tenant_bucket, tenant_id, changeset_id, candidate_metadata_version_id) on delete cascade,
	foreign key (tenant_bucket, tenant_id, candidate_metadata_version_id, object_id)
		references object_definition(tenant_bucket, tenant_id, metadata_version_id, object_id)
);

create index metadata_changeset_state_idx
	on metadata_changeset(tenant_bucket, tenant_id, state, updated_at desc);
create index metadata_changeset_object_state_idx
	on metadata_changeset_object(tenant_bucket, tenant_id, changeset_id, state, object_id);

alter table metadata_changeset enable row level security;
alter table metadata_changeset force row level security;
alter table metadata_changeset_object enable row level security;
alter table metadata_changeset_object force row level security;

create policy tenant_isolation on metadata_changeset using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

create policy tenant_isolation on metadata_changeset_object using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

grant select, insert, update, delete on metadata_changeset, metadata_changeset_object to ai_native_runtime;
