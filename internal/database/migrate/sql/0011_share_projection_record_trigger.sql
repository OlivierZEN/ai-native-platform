-- Keep ready rule projections correct for records created after a refresh and
-- for records whose business organization changes during a controlled merge.
-- The projection remains record-to-group only; group membership is evaluated
-- at read time and is never expanded into record-to-user rows.
create or replace function maintain_share_projection_for_record()
returns trigger
language plpgsql
as $$
begin
	if tg_op = 'UPDATE'
		and new.data_organization_id is not distinct from old.data_organization_id
		and new.lifecycle_state is not distinct from old.lifecycle_state then
		return new;
	end if;

	delete from share_projection
	where tenant_bucket = new.tenant_bucket
		and tenant_id = new.tenant_id
		and object_id = new.object_id
		and record_id = new.record_id;

	if new.lifecycle_state = 'active' and new.data_organization_id is not null then
		insert into share_projection(tenant_bucket,tenant_id,object_id,record_id,group_id,access_level,rule_id)
		select new.tenant_bucket,new.tenant_id,new.object_id,new.record_id,rule.grantee_group_id,rule.access_level,rule.rule_id
		from sharing_rule_def rule
		where rule.tenant_bucket = new.tenant_bucket
			and rule.tenant_id = new.tenant_id
			and rule.object_id = new.object_id
			and rule.lifecycle_state = 'active'
			and rule.projection_state = 'ready'
			and (rule.condition_expression->>'data_organization_id')::uuid = new.data_organization_id
		on conflict (tenant_bucket,tenant_id,object_id,record_id,group_id,rule_id)
		do update set access_level=excluded.access_level,projected_at=clock_timestamp();
	end if;
	return new;
end;
$$;

create trigger object_record_share_projection_maintain
after insert or update of data_organization_id,lifecycle_state on object_record
for each row execute function maintain_share_projection_for_record();
