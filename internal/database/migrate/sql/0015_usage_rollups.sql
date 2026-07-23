create table metering.tenant_usage_hourly (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	hour_started_at timestamptz not null,
	request_count bigint not null default 0,
	execution_count bigint not null default 0,
	ru_milli bigint not null default 0,
	record_count_delta bigint not null default 0,
	logical_data_bytes_delta bigint not null default 0,
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, hour_started_at),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
);
alter table metering.tenant_usage_hourly enable row level security;
alter table metering.tenant_usage_hourly force row level security;
create policy tenant_isolation on metering.tenant_usage_hourly using (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint) with check (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint);
grant select, insert, update on metering.tenant_usage_hourly to ai_native_runtime;
grant select on metering.tenant_usage_hourly to ai_native_control;
