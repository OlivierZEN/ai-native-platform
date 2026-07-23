alter table organization_node
	add column merged_into_organization_id uuid,
	add constraint organization_node_merged_into_fk
		foreign key (tenant_bucket,tenant_id,merged_into_organization_id)
		references organization_node(tenant_bucket,tenant_id,organization_id),
	add constraint organization_node_merged_target_check
		check (merged_into_organization_id is null or merged_into_organization_id <> organization_id);

create table organization_merge_operation (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	operation_id uuid not null,
	source_organization_id uuid not null,
	target_organization_id uuid not null,
	state varchar(24) not null check (state in ('running','completed','cancelled','failed')),
	approval_id text not null,
	initiated_by text not null,
	records_migrated bigint not null default 0 check (records_migrated >= 0),
	last_record_id uuid,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	completed_at timestamptz,
	primary key (tenant_bucket,tenant_id,operation_id),
	foreign key (tenant_bucket,tenant_id,source_organization_id) references organization_node(tenant_bucket,tenant_id,organization_id),
	foreign key (tenant_bucket,tenant_id,target_organization_id) references organization_node(tenant_bucket,tenant_id,organization_id),
	check (source_organization_id <> target_organization_id)
);

create unique index organization_merge_one_running_source_idx
	on organization_merge_operation(tenant_bucket,tenant_id,source_organization_id)
	where state = 'running';

create table record_organization_history (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	history_id uuid not null,
	operation_id uuid not null,
	object_id uuid not null,
	record_id uuid not null,
	from_organization_id uuid not null,
	to_organization_id uuid not null,
	changed_by text not null,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,history_id),
	unique (tenant_bucket,tenant_id,operation_id,object_id,record_id),
	foreign key (tenant_bucket,tenant_id,operation_id) references organization_merge_operation(tenant_bucket,tenant_id,operation_id),
	foreign key (tenant_bucket,tenant_id,object_id,record_id) references object_record(tenant_bucket,tenant_id,object_id,record_id),
	foreign key (tenant_bucket,tenant_id,from_organization_id) references organization_node(tenant_bucket,tenant_id,organization_id),
	foreign key (tenant_bucket,tenant_id,to_organization_id) references organization_node(tenant_bucket,tenant_id,organization_id)
);

create index record_organization_history_operation_idx
	on record_organization_history(tenant_bucket,tenant_id,operation_id,record_id);

alter table organization_merge_operation enable row level security;
alter table organization_merge_operation force row level security;
alter table record_organization_history enable row level security;
alter table record_organization_history force row level security;

create policy tenant_isolation on organization_merge_operation using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);
create policy tenant_isolation on record_organization_history using (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
) with check (
	tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
	and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint
);

grant select, insert, update, delete on organization_merge_operation, record_organization_history to ai_native_runtime;
