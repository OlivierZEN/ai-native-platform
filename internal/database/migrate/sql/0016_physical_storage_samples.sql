create table metering.physical_storage_sample (
	sample_id uuid primary key,
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_record_bytes bigint not null check (object_record_bytes >= 0),
	record_relation_bytes bigint not null check (record_relation_bytes >= 0),
	usage_ledger_bytes bigint not null check (usage_ledger_bytes >= 0),
	captured_at timestamptz not null default clock_timestamp(),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
);
create index physical_storage_sample_tenant_time_idx on metering.physical_storage_sample(tenant_bucket,tenant_id,captured_at desc);
alter table metering.physical_storage_sample enable row level security;
alter table metering.physical_storage_sample force row level security;
create policy tenant_isolation on metering.physical_storage_sample using (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint) with check (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint);
grant select, insert on metering.physical_storage_sample to ai_native_runtime;
grant select on metering.physical_storage_sample to ai_native_control;
