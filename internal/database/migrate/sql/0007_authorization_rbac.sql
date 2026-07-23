-- FEAT-016: role-centred RBAC and organization-scoped record authorization.
-- Object policies are disabled by default so existing objects retain their
-- behaviour until their roles and permissions have been migrated.

create table principal_projection (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	principal_id text not null check (octet_length(principal_id) between 1 and 200),
	principal_type varchar(24) not null check (principal_type in ('user','service','group')),
	status varchar(24) not null default 'active' check (status in ('active','suspended','disabled')),
	identity_version bigint not null default 1 check (identity_version > 0),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,principal_id),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

create table organization_node (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	organization_id uuid not null,
	parent_organization_id uuid,
	name varchar(200) not null,
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','migrating','merged','retired')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,organization_id),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id),
	foreign key (tenant_bucket,tenant_id,parent_organization_id)
		references organization_node(tenant_bucket,tenant_id,organization_id),
	check (parent_organization_id is null or parent_organization_id <> organization_id)
);

create table organization_closure (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	ancestor_organization_id uuid not null,
	descendant_organization_id uuid not null,
	depth integer not null check (depth >= 0),
	primary key (tenant_bucket,tenant_id,ancestor_organization_id,descendant_organization_id),
	foreign key (tenant_bucket,tenant_id,ancestor_organization_id)
		references organization_node(tenant_bucket,tenant_id,organization_id) on delete restrict,
	foreign key (tenant_bucket,tenant_id,descendant_organization_id)
		references organization_node(tenant_bucket,tenant_id,organization_id) on delete restrict,
	check ((depth = 0) = (ancestor_organization_id = descendant_organization_id))
);

create index organization_closure_descendant_idx
	on organization_closure(tenant_bucket,tenant_id,descendant_organization_id,ancestor_organization_id);

create table principal_org_membership (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	membership_id uuid not null,
	principal_id text not null,
	organization_id uuid not null,
	membership_state varchar(24) not null default 'active' check (membership_state in ('active','ended')),
	is_primary boolean not null default false,
	effective_from timestamptz not null default clock_timestamp(),
	effective_to timestamptz,
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,membership_id),
	foreign key (tenant_bucket,tenant_id,principal_id)
		references principal_projection(tenant_bucket,tenant_id,principal_id),
	foreign key (tenant_bucket,tenant_id,organization_id)
		references organization_node(tenant_bucket,tenant_id,organization_id),
	check ((membership_state = 'active' and effective_to is null) or (membership_state = 'ended' and effective_to is not null))
);

create unique index principal_org_membership_one_primary_idx
	on principal_org_membership(tenant_bucket,tenant_id,principal_id)
	where membership_state = 'active' and is_primary;

create table access_group (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	group_id uuid not null,
	name varchar(200) not null,
	group_type varchar(24) not null default 'manual' check (group_type in ('manual','rule')),
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','disabled')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,group_id),
	unique (tenant_bucket,tenant_id,name),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

create table group_membership (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	group_id uuid not null,
	principal_id text not null,
	membership_state varchar(24) not null default 'active' check (membership_state in ('active','ended')),
	created_at timestamptz not null default clock_timestamp(),
	ended_at timestamptz,
	primary key (tenant_bucket,tenant_id,group_id,principal_id),
	foreign key (tenant_bucket,tenant_id,group_id) references access_group(tenant_bucket,tenant_id,group_id),
	foreign key (tenant_bucket,tenant_id,principal_id) references principal_projection(tenant_bucket,tenant_id,principal_id),
	check ((membership_state = 'active' and ended_at is null) or (membership_state = 'ended' and ended_at is not null))
);

create table authorization_role (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	role_id uuid not null,
	name varchar(200) not null,
	description text not null default '',
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','disabled')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,role_id),
	unique (tenant_bucket,tenant_id,name),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

create table permission_set (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	permission_set_id uuid not null,
	name varchar(200) not null,
	description text not null default '',
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','disabled')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,permission_set_id),
	unique (tenant_bucket,tenant_id,name),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

-- resource_ref is an object/field UUID encoded as text, or '*' for a platform-wide permission.
create table authorization_permission (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	permission_id uuid not null,
	resource_type varchar(32) not null check (resource_type in ('platform','object','field')),
	resource_ref text not null,
	action varchar(32) not null,
	effect varchar(12) not null default 'allow' check (effect = 'allow'),
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,permission_id),
	unique (tenant_bucket,tenant_id,resource_type,resource_ref,action,effect),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

create table permission_set_permission (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	permission_set_id uuid not null,
	permission_id uuid not null,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,permission_set_id,permission_id),
	foreign key (tenant_bucket,tenant_id,permission_set_id) references permission_set(tenant_bucket,tenant_id,permission_set_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,permission_id) references authorization_permission(tenant_bucket,tenant_id,permission_id) on delete cascade
);

create table role_permission_set (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	role_id uuid not null,
	permission_set_id uuid not null,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,role_id,permission_set_id),
	foreign key (tenant_bucket,tenant_id,role_id) references authorization_role(tenant_bucket,tenant_id,role_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,permission_set_id) references permission_set(tenant_bucket,tenant_id,permission_set_id) on delete cascade
);

create table principal_role_assignment (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	assignment_id uuid not null,
	principal_id text not null,
	role_id uuid not null,
	assignment_state varchar(24) not null default 'active' check (assignment_state in ('active','ended')),
	effective_from timestamptz not null default clock_timestamp(),
	effective_to timestamptz,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,assignment_id),
	foreign key (tenant_bucket,tenant_id,principal_id) references principal_projection(tenant_bucket,tenant_id,principal_id),
	foreign key (tenant_bucket,tenant_id,role_id) references authorization_role(tenant_bucket,tenant_id,role_id),
	check ((assignment_state = 'active' and effective_to is null) or (assignment_state = 'ended' and effective_to is not null))
);

create index principal_role_assignment_active_idx
	on principal_role_assignment(tenant_bucket,tenant_id,principal_id,role_id)
	where assignment_state = 'active';

create table role_data_scope (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	scope_id uuid not null,
	role_id uuid not null,
	object_id uuid not null,
	action varchar(32) not null check (action in ('read','update','delete')),
	scope_type varchar(32) not null check (scope_type in ('own','organization','organization_descendants','assigned_organizations','all_tenant','conditional')),
	organization_id uuid,
	condition_expression jsonb,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,scope_id),
	unique (tenant_bucket,tenant_id,role_id,object_id,action,scope_type,organization_id),
	foreign key (tenant_bucket,tenant_id,role_id) references authorization_role(tenant_bucket,tenant_id,role_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,organization_id) references organization_node(tenant_bucket,tenant_id,organization_id),
	check ((scope_type in ('organization','organization_descendants') and organization_id is not null) or (scope_type not in ('organization','organization_descendants') and organization_id is null)),
	check ((scope_type = 'conditional') = (condition_expression is not null))
);

create table role_conflict (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	role_id uuid not null,
	conflicting_role_id uuid not null,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,role_id,conflicting_role_id),
	foreign key (tenant_bucket,tenant_id,role_id) references authorization_role(tenant_bucket,tenant_id,role_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,conflicting_role_id) references authorization_role(tenant_bucket,tenant_id,role_id) on delete cascade,
	check (role_id <> conflicting_role_id)
);

create table object_authorization_policy (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid not null,
	enforcement_state varchar(16) not null default 'disabled' check (enforcement_state in ('disabled','enforced')),
	default_record_access varchar(24) not null default 'private' check (default_record_access in ('private','read_all')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,object_id),
	foreign key (tenant_bucket,tenant_id) references tenant_registry(tenant_bucket,tenant_id)
);

alter table object_record
	add column data_organization_id uuid,
	add constraint object_record_data_organization_fk
		foreign key (tenant_bucket,tenant_id,data_organization_id)
		references organization_node(tenant_bucket,tenant_id,organization_id);

create index object_record_data_organization_idx
	on object_record(tenant_bucket,tenant_id,object_id,data_organization_id,record_id)
	where lifecycle_state = 'active' and data_organization_id is not null;

create table record_team_member (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid not null,
	record_id uuid not null,
	principal_id text not null,
	access_level varchar(16) not null check (access_level in ('read','update','delete')),
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','revoked')),
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,object_id,record_id,principal_id,access_level),
	foreign key (tenant_bucket,tenant_id,object_id,record_id) references object_record(tenant_bucket,tenant_id,object_id,record_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,principal_id) references principal_projection(tenant_bucket,tenant_id,principal_id)
);

create table share_grant (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	share_grant_id uuid not null,
	object_id uuid not null,
	record_id uuid not null,
	grantee_type varchar(16) not null check (grantee_type in ('principal','group')),
	grantee_ref text not null,
	access_level varchar(16) not null check (access_level in ('read','update','delete')),
	grant_source varchar(24) not null check (grant_source in ('manual','rule','migration')),
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','revoked')),
	expires_at timestamptz,
	created_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,share_grant_id),
	foreign key (tenant_bucket,tenant_id,object_id,record_id) references object_record(tenant_bucket,tenant_id,object_id,record_id) on delete cascade
);

create index share_grant_grantee_lookup_idx
	on share_grant(tenant_bucket,tenant_id,object_id,record_id,grantee_type,grantee_ref,access_level)
	where lifecycle_state = 'active';

create table sharing_rule_def (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	rule_id uuid not null,
	object_id uuid not null,
	name varchar(200) not null,
	condition_expression jsonb not null,
	grantee_group_id uuid not null,
	access_level varchar(16) not null check (access_level in ('read','update','delete')),
	lifecycle_state varchar(24) not null default 'active' check (lifecycle_state in ('active','disabled')),
	created_at timestamptz not null default clock_timestamp(),
	updated_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,rule_id),
	unique (tenant_bucket,tenant_id,object_id,name),
	foreign key (tenant_bucket,tenant_id,grantee_group_id) references access_group(tenant_bucket,tenant_id,group_id)
);

-- This optional projection deliberately stores record-to-group edges only; it
-- never expands into a record-to-user ACL table.
create table share_projection (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	object_id uuid not null,
	record_id uuid not null,
	group_id uuid not null,
	access_level varchar(16) not null check (access_level in ('read','update','delete')),
	rule_id uuid not null,
	projected_at timestamptz not null default clock_timestamp(),
	primary key (tenant_bucket,tenant_id,object_id,record_id,group_id,rule_id),
	foreign key (tenant_bucket,tenant_id,object_id,record_id) references object_record(tenant_bucket,tenant_id,object_id,record_id) on delete cascade,
	foreign key (tenant_bucket,tenant_id,group_id) references access_group(tenant_bucket,tenant_id,group_id),
	foreign key (tenant_bucket,tenant_id,rule_id) references sharing_rule_def(tenant_bucket,tenant_id,rule_id) on delete cascade
);

create table permission_snapshot (
	tenant_bucket smallint not null check (tenant_bucket between 0 and 127),
	tenant_id uuid not null,
	principal_id text not null,
	snapshot_version bigint not null check (snapshot_version > 0),
	computed_at timestamptz not null default clock_timestamp(),
	expires_at timestamptz,
	payload jsonb not null,
	primary key (tenant_bucket,tenant_id,principal_id),
	foreign key (tenant_bucket,tenant_id,principal_id) references principal_projection(tenant_bucket,tenant_id,principal_id)
);

do $$
declare
	table_name text;
begin
	foreach table_name in array array[
		'principal_projection','organization_node','organization_closure','principal_org_membership',
		'access_group','group_membership','authorization_role','permission_set','authorization_permission',
		'permission_set_permission','role_permission_set','principal_role_assignment','role_data_scope',
		'role_conflict','object_authorization_policy','record_team_member','share_grant','sharing_rule_def',
		'share_projection','permission_snapshot'
	]
	loop
		execute format('alter table %I enable row level security', table_name);
		execute format('alter table %I force row level security', table_name);
		execute format(
			'create policy tenant_isolation on %I using (
				tenant_id = nullif(current_setting(''app.tenant_id'', true), '''')::uuid
				and tenant_bucket = nullif(current_setting(''app.tenant_bucket'', true), '''')::smallint
			) with check (
				tenant_id = nullif(current_setting(''app.tenant_id'', true), '''')::uuid
				and tenant_bucket = nullif(current_setting(''app.tenant_bucket'', true), '''')::smallint
			)', table_name
		);
	end loop;
end
$$;

grant select, insert, update, delete on
	principal_projection, organization_node, organization_closure, principal_org_membership,
	access_group, group_membership, authorization_role, permission_set, authorization_permission,
	permission_set_permission, role_permission_set, principal_role_assignment, role_data_scope,
	role_conflict, object_authorization_policy, record_team_member, share_grant, sharing_rule_def,
	share_projection, permission_snapshot
to ai_native_runtime;
