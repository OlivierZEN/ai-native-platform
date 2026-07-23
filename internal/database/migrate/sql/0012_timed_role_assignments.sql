-- Active role assignments may be explicitly time-bounded. Evaluators treat a
-- past effective_to as absent even if a background lifecycle worker has not
-- yet changed assignment_state to ended.
alter table principal_role_assignment
	drop constraint principal_role_assignment_check;

alter table principal_role_assignment
	add constraint principal_role_assignment_check check (
		(assignment_state = 'active' and (effective_to is null or effective_to > effective_from))
		or (assignment_state = 'ended' and effective_to is not null)
	);

create index principal_role_assignment_expiry_idx
	on principal_role_assignment(tenant_bucket,tenant_id,effective_to)
	where assignment_state = 'active' and effective_to is not null;
