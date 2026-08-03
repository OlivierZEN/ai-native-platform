package console

import (
	"context"
	"fmt"
	"time"

	"github.com/OlivierZEN/ai-native-platform/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reader returns governance data only after Handler has verified the console
// session. Implementations must derive tenant scope from that session, never
// from request parameters supplied by the browser.
type Reader interface {
	Read(context.Context, session, string) (any, error)
}

type PostgresReader struct {
	runtime *pgxpool.Pool
	control *pgxpool.Pool
}

type tenantProjection struct {
	tenantID          uuid.UUID
	bucket            int16
	companyID         string
	displayName       string
	metadataVersionID uuid.UUID
}

func NewPostgresReader(runtime, control *pgxpool.Pool) *PostgresReader {
	if runtime == nil || control == nil {
		panic("console reader requires runtime and control database pools")
	}
	return &PostgresReader{runtime: runtime, control: control}
}

func (reader *PostgresReader) Read(ctx context.Context, s session, path string) (any, error) {
	tenant, err := reader.resolveTenant(ctx, s)
	if err != nil {
		return nil, err
	}
	var value any
	err = database.WithTenant(ctx, reader.runtime, database.TenantContext{TenantID: tenant.tenantID, Bucket: tenant.bucket, ActorID: s.Subject}, func(tx pgx.Tx) error {
		switch path {
		case "/console/api/overview":
			value, err = reader.overview(ctx, tx, tenant)
		case "/console/api/members":
			value, err = reader.members(ctx, tx, tenant)
		case "/console/api/organizations":
			value, err = reader.organizations(ctx, tx, tenant)
		case "/console/api/objects":
			value, err = reader.objects(ctx, tx, tenant)
		case "/console/api/audit-events":
			value, err = reader.auditEvents(ctx, tx, tenant)
		case "/console/api/system-settings":
			value, err = reader.settings(ctx, tx, tenant)
		default:
			return fmt.Errorf("unknown console data route")
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (reader *PostgresReader) resolveTenant(ctx context.Context, s session) (tenantProjection, error) {
	tenantID, err := uuid.Parse(s.TenantID)
	if err != nil || tenantID == uuid.Nil || s.CompanyID == "" || s.Subject == "" {
		return tenantProjection{}, fmt.Errorf("invalid verified console tenant context")
	}
	var tenant tenantProjection
	err = reader.control.QueryRow(ctx, `
		select tenant_id, tenant_bucket, company_id, display_name,
		       coalesce(metadata_version_id, '00000000-0000-0000-0000-000000000000'::uuid)
		from tenant_registry
		where tenant_id=$1 and company_id=$2 and native_status='active'`, tenantID, s.CompanyID).
		Scan(&tenant.tenantID, &tenant.bucket, &tenant.companyID, &tenant.displayName, &tenant.metadataVersionID)
	if err != nil {
		return tenantProjection{}, fmt.Errorf("active console tenant projection was not found")
	}
	return tenant, nil
}

func base(tenant tenantProjection) map[string]any {
	return map[string]any{
		"environment": "已发布租户数据",
		"data_notice": "Semattice 已发布研发交付模型 · 只读",
		"company_id":  tenant.companyID,
		"tenant_name": tenant.displayName,
	}
}

func (reader *PostgresReader) overview(ctx context.Context, tx pgx.Tx, tenant tenantProjection) (any, error) {
	result := base(tenant)
	var members, roles, organizations, objects, fields, auditEvents int
	if err := tx.QueryRow(ctx, `select count(*) from principal_projection`).Scan(&members); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from authorization_role where lifecycle_state='active'`).Scan(&roles); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from organization_node where lifecycle_state in ('active','migrating')`).Scan(&organizations); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `select count(*) from audit_event`).Scan(&auditEvents); err != nil {
		return nil, err
	}
	if tenant.metadataVersionID != uuid.Nil {
		if err := tx.QueryRow(ctx, `select count(*) from object_definition where metadata_version_id=$1`, tenant.metadataVersionID).Scan(&objects); err != nil {
			return nil, err
		}
		if err := tx.QueryRow(ctx, `select count(*) from field_definition where metadata_version_id=$1 and lifecycle_state <> 'tombstone'`, tenant.metadataVersionID).Scan(&fields); err != nil {
			return nil, err
		}
	}
	result["summary"] = map[string]int{"members": members, "roles": roles, "organizations": organizations, "objects": objects, "fields": fields, "audit_events": auditEvents}
	result["recent"] = reader.recentAudit(ctx, tx)
	return result, nil
}

func (reader *PostgresReader) recentAudit(ctx context.Context, tx pgx.Tx) []map[string]string {
	rows, err := tx.Query(ctx, `select capability_id, status, created_at from audit_event order by created_at desc limit 3`)
	if err != nil {
		return []map[string]string{}
	}
	defer rows.Close()
	result := make([]map[string]string, 0)
	for rows.Next() {
		var capabilityID, status string
		var createdAt time.Time
		if rows.Scan(&capabilityID, &status, &createdAt) == nil {
			result = append(result, map[string]string{"title": capabilityID, "type": status, "time": createdAt.UTC().Format("2006-01-02 15:04")})
		}
	}
	return result
}

func (reader *PostgresReader) members(ctx context.Context, tx pgx.Tx, tenant tenantProjection) (any, error) {
	result := base(tenant)
	rows, err := tx.Query(ctx, `
		select coalesce(nullif(p.display_name,''),p.principal_id),
		       case when p.principal_type='service' then coalesce(nullif(p.client_id,''),'机器主体')
		            else coalesce(nullif(p.public_id,''),'AgentCiCi 全局账号') end,
		       coalesce(string_agg(distinct r.name, ' · '), '未分配角色'),
		       coalesce(max(o.name), '未分配组织'), p.status,
		       case when p.principal_type='service'
		            then coalesce(max(nullif(owner.display_name,'')),p.owner_principal_id,'未绑定负责人')
		            else '本人' end
		from principal_projection p
		left join principal_projection owner on owner.principal_id=p.owner_principal_id
		left join principal_role_assignment a on a.principal_id=p.principal_id and a.assignment_state='active'
		left join authorization_role r on r.role_id=a.role_id and r.lifecycle_state='active'
		left join principal_org_membership m on m.principal_id=p.principal_id and m.membership_state='active' and m.is_primary
		left join organization_node o on o.organization_id=m.organization_id
		group by p.principal_id,p.principal_type,p.display_name,p.public_id,p.client_id,p.owner_principal_id,p.status,p.created_at
		order by p.created_at,p.principal_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := make([]map[string]any, 0)
	for rows.Next() {
		var name, account, role, organization, status, owner string
		if err := rows.Scan(&name, &account, &role, &organization, &status, &owner); err != nil {
			return nil, err
		}
		members = append(members, map[string]any{"name": name, "account": account, "role": role, "organization": organization, "status": status, "owner": owner})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	roleRows, err := tx.Query(ctx, `
		select r.name, coalesce(string_agg(distinct p.resource_type || '.' || p.action, ' · '), '未绑定权限')
		from authorization_role r
		left join role_permission_set rp on rp.role_id=r.role_id
		left join permission_set_permission pp on pp.permission_set_id=rp.permission_set_id
		left join authorization_permission p on p.permission_id=pp.permission_id
		where r.lifecycle_state='active'
		group by r.role_id,r.name order by r.name`)
	if err != nil {
		return nil, err
	}
	defer roleRows.Close()
	roles := make([]map[string]string, 0)
	for roleRows.Next() {
		var name, scope string
		if err := roleRows.Scan(&name, &scope); err != nil {
			return nil, err
		}
		roles = append(roles, map[string]string{"name": name, "scope": scope})
	}
	if err := roleRows.Err(); err != nil {
		return nil, err
	}
	result["members"] = members
	result["roles"] = roles
	result["empty_notice"] = "当前租户尚未投影 Semattice 本地成员、角色或组织；统一身份与授权仍由 AgentCiCi 管理。"
	return result, nil
}

func (reader *PostgresReader) organizations(ctx context.Context, tx pgx.Tx, tenant tenantProjection) (any, error) {
	result := base(tenant)
	rows, err := tx.Query(ctx, `
		select o.name, coalesce((select max(depth) from organization_closure c where c.descendant_organization_id=o.organization_id), 0),
		       (select count(*) from principal_org_membership m where m.organization_id=o.organization_id and m.membership_state='active'), o.lifecycle_state
		from organization_node o order by o.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := make([]map[string]any, 0)
	for rows.Next() {
		var name, level, members, state string
		if err := rows.Scan(&name, &level, &members, &state); err != nil {
			return nil, err
		}
		nodes = append(nodes, map[string]any{"name": name, "level": level, "members": members, "state": state})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result["nodes"] = nodes
	result["empty_notice"] = "当前租户尚未投影 Semattice 本地组织架构。"
	return result, nil
}

func (reader *PostgresReader) objects(ctx context.Context, tx pgx.Tx, tenant tenantProjection) (any, error) {
	result := base(tenant)
	objects := make([]map[string]any, 0)
	if tenant.metadataVersionID == uuid.Nil {
		result["objects"] = objects
		result["selected"] = ""
		return result, nil
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `select sequence from metadata_version where metadata_version_id=$1 and status='published'`, tenant.metadataVersionID).Scan(&sequence); err != nil {
		return nil, err
	}
	type objectRow struct {
		databaseID, apiName, label, description string
		related                                 int
	}
	rows, err := tx.Query(ctx, `
		select o.object_id::text,o.api_name,o.label,o.description,
		       count(distinct r.relation_id)
		from object_definition o
		left join relation_definition r on r.metadata_version_id=o.metadata_version_id and (r.source_object_id=o.object_id or r.target_object_id=o.object_id)
		where o.metadata_version_id=$1
		group by o.object_id,o.api_name,o.label,o.description order by o.api_name`, tenant.metadataVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objectRows := make([]objectRow, 0)
	for rows.Next() {
		var row objectRow
		if err := rows.Scan(&row.databaseID, &row.apiName, &row.label, &row.description, &row.related); err != nil {
			return nil, err
		}
		objectRows = append(objectRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	fieldsByObject := map[string][]map[string]string{}
	fieldRows, err := tx.Query(ctx, `
		select object_id::text,label,api_name,data_type,required,unique_value,indexed,
		       coalesce(nullif(semantic->>'classification',''),'未标注')
		from field_definition where metadata_version_id=$1 and lifecycle_state <> 'tombstone'
		order by object_id,api_name`, tenant.metadataVersionID)
	if err != nil {
		return nil, err
	}
	defer fieldRows.Close()
	for fieldRows.Next() {
		var objectID, label, apiName, dataType, classification string
		var required, uniqueValue, indexed bool
		if err := fieldRows.Scan(&objectID, &label, &apiName, &dataType, &required, &uniqueValue, &indexed, &classification); err != nil {
			return nil, err
		}
		fieldsByObject[objectID] = append(fieldsByObject[objectID], map[string]string{"name": label, "key": apiName, "type": dataType, "required": yesNo(required), "unique": yesNo(uniqueValue), "indexed": yesNo(indexed), "classification": classification})
	}
	if err := fieldRows.Err(); err != nil {
		return nil, err
	}
	for _, row := range objectRows {
		objects = append(objects, map[string]any{
			"id": row.apiName, "name": row.label, "label": "已发布研发交付模型", "version": fmt.Sprintf("v%d", sequence), "fields": objectFields(fieldsByObject[row.databaseID]),
			"inspector": map[string]any{"source": "Semattice 已发布元数据", "owner": "受控发布", "related": row.related, "status": "已发布", "description": row.description},
		})
	}
	result["objects"] = objects
	if len(objects) > 0 {
		result["selected"] = objects[0]["id"]
	}
	return result, nil
}

func objectFields(fields []map[string]string) []map[string]string {
	if fields == nil {
		return make([]map[string]string, 0)
	}
	return fields
}

func (reader *PostgresReader) auditEvents(ctx context.Context, tx pgx.Tx, tenant tenantProjection) (any, error) {
	result := base(tenant)
	rows, err := tx.Query(ctx, `select created_at,actor_id,capability_id,status from audit_event order by created_at desc limit 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]map[string]string, 0)
	for rows.Next() {
		var createdAt time.Time
		var actor, action, status string
		if err := rows.Scan(&createdAt, &actor, &action, &status); err != nil {
			return nil, err
		}
		events = append(events, map[string]string{"time": createdAt.UTC().Format("2006-01-02 15:04:05"), "actor": actor, "action": action, "target": "当前租户", "result": status})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result["events"] = events
	return result, nil
}

func (reader *PostgresReader) settings(ctx context.Context, tx pgx.Tx, tenant tenantProjection) (any, error) {
	result := base(tenant)
	objects, fields := 0, 0
	if tenant.metadataVersionID != uuid.Nil {
		if err := tx.QueryRow(ctx, `select count(*) from object_definition where metadata_version_id=$1`, tenant.metadataVersionID).Scan(&objects); err != nil {
			return nil, err
		}
		if err := tx.QueryRow(ctx, `select count(*) from field_definition where metadata_version_id=$1 and lifecycle_state <> 'tombstone'`, tenant.metadataVersionID).Scan(&fields); err != nil {
			return nil, err
		}
	}
	result["settings"] = []map[string]string{
		{"group": "数据治理", "name": "已发布研发交付模型", "value": fmt.Sprintf("%d 个对象 · %d 个有效字段", objects, fields), "state": "已发布"},
		{"group": "运行时", "name": "API 认证", "value": "官方 OACT / 最小 scope", "state": "正常"},
		{"group": "安全", "name": "浏览器会话", "value": "短时 HttpOnly Cookie", "state": "正常"},
	}
	return result, nil
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}
