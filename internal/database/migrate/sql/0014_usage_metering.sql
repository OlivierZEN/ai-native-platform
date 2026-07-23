create schema metering;

create table metering.usage_ledger_event (
	event_id uuid primary key,
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid,
	request_id text not null,
	capability_id text not null,
	entrypoint varchar(8) not null check (entrypoint in ('api','mcp','cli','internal')),
	event_kind varchar(24) not null check (event_kind in ('invocation','record_delta')),
	status varchar(24) not null,
	executed_count integer not null default 0 check (executed_count between 0 and 1),
	ru_milli bigint not null default 0 check (ru_milli >= 0),
	record_count_delta bigint not null default 0,
	logical_data_bytes_delta bigint not null default 0,
	duration_ms integer not null default 0 check (duration_ms >= 0),
	input_bytes integer not null default 0 check (input_bytes >= 0),
	output_bytes integer not null default 0 check (output_bytes >= 0),
	meter_version varchar(32) not null,
	occurred_at timestamptz not null default clock_timestamp(),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
);
create unique index usage_ledger_invocation_request_idx on metering.usage_ledger_event(tenant_bucket, tenant_id, request_id, event_kind) where event_kind = 'invocation';
create index usage_ledger_tenant_time_idx on metering.usage_ledger_event(tenant_bucket, tenant_id, occurred_at desc);

create table metering.tenant_usage_current_bucket (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid not null,
	counter_bucket smallint not null check (counter_bucket between 0 and 15),
	live_record_count bigint not null default 0 check (live_record_count >= 0),
	logical_data_bytes bigint not null default 0 check (logical_data_bytes >= 0),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket, tenant_id, object_id, counter_bucket),
	foreign key (tenant_bucket, tenant_id) references tenant_registry(tenant_bucket, tenant_id)
);

-- Seed pre-metering records so the first update/delete after upgrade cannot
-- drive a counter below zero. New mutations maintain the same compact logical
-- JSON representation through the application transaction.
insert into metering.tenant_usage_current_bucket(tenant_bucket,tenant_id,object_id,counter_bucket,live_record_count,logical_data_bytes)
select tenant_bucket,tenant_id,object_id,(get_byte(uuid_send(record_id),15) % 16)::smallint,count(*),coalesce(sum(octet_length(data::text)),0)
from object_record
where lifecycle_state='active'
group by tenant_bucket,tenant_id,object_id,(get_byte(uuid_send(record_id),15) % 16)::smallint;

alter table metering.usage_ledger_event enable row level security;
alter table metering.usage_ledger_event force row level security;
alter table metering.tenant_usage_current_bucket enable row level security;
alter table metering.tenant_usage_current_bucket force row level security;
create policy tenant_isolation on metering.usage_ledger_event using (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint) with check (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint);
create policy tenant_isolation on metering.tenant_usage_current_bucket using (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint) with check (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid and tenant_bucket = nullif(current_setting('app.tenant_bucket', true), '')::smallint);
grant usage on schema metering to ai_native_runtime, ai_native_control;
grant select, insert on metering.usage_ledger_event to ai_native_runtime;
grant select, insert, update on metering.tenant_usage_current_bucket to ai_native_runtime;
grant select on metering.usage_ledger_event, metering.tenant_usage_current_bucket to ai_native_control;
