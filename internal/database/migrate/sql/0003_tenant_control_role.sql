do $$
begin
	if not exists (select 1 from pg_roles where rolname = 'ai_native_control') then
		create role ai_native_control nologin nosuperuser nocreatedb nocreaterole noinherit nobypassrls;
	end if;
end
$$;

grant usage on schema public to ai_native_control;
grant select, insert, update on tenant_registry to ai_native_control;
grant select, insert, update on tenant_operation to ai_native_control;
grant select, insert on audit_event to ai_native_control;

create policy tenant_control_access on tenant_registry
	for all to ai_native_control using (true) with check (true);
create policy tenant_control_access on tenant_operation
	for all to ai_native_control using (true) with check (true);
create policy tenant_control_access on audit_event
	for all to ai_native_control using (true) with check (true);
