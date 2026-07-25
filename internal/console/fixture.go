package console

func fixture(path string, s session) any {
	base := map[string]any{"environment": "演示环境", "data_notice": "模拟治理数据，不写入业务数据库", "company_id": s.CompanyID}
	switch path {
	case "/console/api/overview":
		base["summary"] = map[string]int{"members": 8, "roles": 5, "organizations": 6, "objects": 12, "fields": 84, "audit_events": 24}
		base["recent"] = []map[string]string{{"title": "对象版本 v1.6 已发布", "time": "10:42", "type": "metadata"}, {"title": "权限快照已完成核验", "time": "09:18", "type": "security"}, {"title": "运行时健康检查通过", "time": "08:55", "type": "runtime"}}
	case "/console/api/members":
		base["members"] = []map[string]any{{"name": "周芮", "account": "zhou.rui@example.demo", "role": "系统管理员", "organization": "数据治理部", "status": "正常"}, {"name": "林川", "account": "lin.chuan@example.demo", "role": "权限管理员", "organization": "平台工程部", "status": "正常"}, {"name": "陈微", "account": "chen.wei@example.demo", "role": "安全审计员", "organization": "风险与合规部", "status": "正常"}, {"name": "宋彦", "account": "song.yan@example.demo", "role": "应用管理员", "organization": "业务运营部", "status": "已暂停"}, {"name": "唐昕", "account": "tang.xin@example.demo", "role": "组织管理员", "organization": "数据治理部", "status": "正常"}, {"name": "顾宁", "account": "gu.ning@example.demo", "role": "配置审阅员", "organization": "风险与合规部", "status": "正常"}, {"name": "叶澜", "account": "ye.lan@example.demo", "role": "应用管理员", "organization": "平台工程部", "status": "正常"}, {"name": "陆衡", "account": "lu.heng@example.demo", "role": "权限管理员", "organization": "平台可靠性组", "status": "正常"}}
		base["roles"] = []map[string]string{{"name": "系统管理员", "scope": "system.manage · authorization.manage"}, {"name": "权限管理员", "scope": "authorization.manage"}, {"name": "组织管理员", "scope": "organization.manage"}, {"name": "安全审计员", "scope": "audit.read"}, {"name": "配置审阅员", "scope": "system.manage（只读）"}}
	case "/console/api/organizations":
		base["nodes"] = []map[string]any{{"name": "演示企业", "level": 0, "members": 8, "state": "正常"}, {"name": "数据治理部", "level": 1, "members": 3, "state": "正常"}, {"name": "平台工程部", "level": 1, "members": 2, "state": "正常"}, {"name": "风险与合规部", "level": 1, "members": 2, "state": "正常"}, {"name": "业务运营部", "level": 1, "members": 1, "state": "调整中"}, {"name": "平台可靠性组", "level": 2, "members": 1, "state": "正常"}}
	case "/console/api/objects":
		base["objects"] = objectFixture()
		base["selected"] = "customer"
	case "/console/api/audit-events":
		base["events"] = auditFixture()
	case "/console/api/system-settings":
		base["settings"] = []map[string]string{{"group": "运行时", "name": "API 认证", "value": "官方 OACT / 最小 scope", "state": "正常"}, {"group": "运行时", "name": "MCP 传输", "value": "Streamable HTTP", "state": "正常"}, {"group": "运行时", "name": "运行时健康检查", "value": "边缘与应用双层探测", "state": "正常"}, {"group": "数据治理", "name": "字段版本策略", "value": "发布后不可变快照", "state": "正常"}, {"group": "数据治理", "name": "对象标识规范", "value": "小写 snake_case", "state": "正常"}, {"group": "数据治理", "name": "敏感字段标识", "value": "分类标签受控展示", "state": "正常"}, {"group": "安全", "name": "会话有效期", "value": "≤ 15 分钟", "state": "正常"}, {"group": "安全", "name": "审计保留", "value": "受控保留策略", "state": "正常"}, {"group": "安全", "name": "浏览器存储", "value": "不存储 OACT", "state": "正常"}, {"group": "集成", "name": "统一身份跳转", "value": "OACT fragment 交换", "state": "正常"}}
	default:
		return map[string]any{"error": "not found"}
	}
	return base
}

func objectFixture() []map[string]any {
	return []map[string]any{
		{"id": "customer", "name": "客户", "label": "核心主数据", "version": "v1.6", "fields": []map[string]string{{"name": "客户编号", "key": "customer_id", "type": "字符串 (36)", "required": "是", "unique": "是", "indexed": "是", "classification": "内部"}, {"name": "客户名称", "key": "customer_name", "type": "字符串 (200)", "required": "是", "unique": "否", "indexed": "是", "classification": "内部"}, {"name": "客户等级", "key": "customer_level", "type": "枚举", "required": "否", "unique": "否", "indexed": "是", "classification": "内部"}, {"name": "统一社会信用代码", "key": "credit_code", "type": "字符串 (18)", "required": "否", "unique": "是", "indexed": "是", "classification": "敏感（企业）"}, {"name": "联系人邮箱", "key": "contact_email", "type": "字符串 (128)", "required": "否", "unique": "否", "indexed": "是", "classification": "敏感（个人）"}, {"name": "创建时间", "key": "created_at", "type": "日期时间", "required": "是", "unique": "否", "indexed": "是", "classification": "系统"}}, "inspector": map[string]any{"source": "主数据域", "owner": "数据架构组", "related": 6, "status": "已发布", "description": "企业客户主数据的语义对象。"}},
		{"id": "service_request", "name": "服务请求", "label": "服务运行对象", "version": "v1.3", "fields": []map[string]string{{"name": "请求编号", "key": "request_id", "type": "字符串 (36)", "required": "是", "unique": "是", "indexed": "是", "classification": "内部"}, {"name": "请求状态", "key": "status", "type": "枚举", "required": "是", "unique": "否", "indexed": "是", "classification": "内部"}}, "inspector": map[string]any{"source": "服务域", "owner": "平台工程组", "related": 4, "status": "已发布", "description": "面向运行审计的服务请求语义对象。"}},
		{"id": "organization", "name": "组织", "label": "权限边界对象", "version": "v1.4", "fields": []map[string]string{{"name": "组织标识", "key": "organization_id", "type": "字符串 (36)", "required": "是", "unique": "是", "indexed": "是", "classification": "系统"}, {"name": "组织名称", "key": "organization_name", "type": "字符串 (128)", "required": "是", "unique": "否", "indexed": "是", "classification": "内部"}}, "inspector": map[string]any{"source": "授权域", "owner": "权限治理组", "related": 3, "status": "已发布", "description": "用于成员与数据范围治理的组织对象。"}},
		{"id": "audit_event", "name": "审计事件", "label": "运行审计对象", "version": "v1.2", "fields": []map[string]string{{"name": "事件标识", "key": "audit_id", "type": "字符串 (36)", "required": "是", "unique": "是", "indexed": "是", "classification": "系统"}, {"name": "发生时间", "key": "occurred_at", "type": "日期时间", "required": "是", "unique": "否", "indexed": "是", "classification": "系统"}}, "inspector": map[string]any{"source": "审计域", "owner": "风险与合规组", "related": 2, "status": "已发布", "description": "运行时治理操作的不可变审计投影。"}},
		metadataObject("access_group", "访问组", "授权治理对象", "v1.1", "授权域", "权限治理组", "用于归集权限边界和适用成员的只读语义对象。"),
		metadataObject("role_assignment", "角色分配", "权限映射对象", "v1.3", "授权域", "权限治理组", "记录角色、成员和组织范围关系的治理对象。"),
		metadataObject("permission_set", "权限集", "权限策略对象", "v1.2", "授权域", "权限治理组", "用于核验最小权限集合与 scope 投影的对象。"),
		metadataObject("capability_contract", "能力契约", "运行时契约对象", "v1.5", "运行时域", "平台工程组", "描述已登记能力及其受控调用边界的对象。"),
		metadataObject("metadata_version", "元数据版本", "版本治理对象", "v1.4", "数据治理域", "数据架构组", "用于追踪对象定义与字段快照版本的对象。"),
		metadataObject("data_classification", "数据分类", "数据治理对象", "v1.2", "数据治理域", "数据架构组", "定义字段可见性与敏感级别标识的对象。"),
		metadataObject("runtime_route", "运行路由", "平台配置对象", "v1.1", "运行时域", "平台可靠性组", "用于核验受控服务路由与健康检查策略的对象。"),
		metadataObject("integration_binding", "集成绑定", "统一身份对象", "v1.3", "集成域", "平台工程组", "描述统一身份交换和内部调用契约的对象。"),
	}
}

func metadataObject(id, name, label, version, source, owner, description string) map[string]any {
	return map[string]any{"id": id, "name": name, "label": label, "version": version, "fields": metadataFields(id), "inspector": map[string]any{"source": source, "owner": owner, "related": 4, "status": "已发布", "description": description}}
}

func metadataFields(prefix string) []map[string]string {
	return []map[string]string{{"name": "唯一标识", "key": prefix + "_id", "type": "字符串 (36)", "required": "是", "unique": "是", "indexed": "是", "classification": "系统"}, {"name": "显示名称", "key": "display_name", "type": "字符串 (128)", "required": "是", "unique": "否", "indexed": "是", "classification": "内部"}, {"name": "状态", "key": "status", "type": "枚举", "required": "是", "unique": "否", "indexed": "是", "classification": "内部"}, {"name": "所属范围", "key": "scope", "type": "字符串 (64)", "required": "否", "unique": "否", "indexed": "是", "classification": "内部"}, {"name": "数据分类", "key": "classification", "type": "枚举", "required": "是", "unique": "否", "indexed": "是", "classification": "内部"}, {"name": "版本号", "key": "version", "type": "字符串 (16)", "required": "是", "unique": "否", "indexed": "是", "classification": "系统"}, {"name": "创建主体", "key": "created_by", "type": "字符串 (64)", "required": "是", "unique": "否", "indexed": "否", "classification": "系统"}, {"name": "创建时间", "key": "created_at", "type": "日期时间", "required": "是", "unique": "否", "indexed": "是", "classification": "系统"}, {"name": "更新时间", "key": "updated_at", "type": "日期时间", "required": "是", "unique": "否", "indexed": "是", "classification": "系统"}}
}

func auditFixture() []map[string]string {
	events := []map[string]string{{"time": "2026-07-25 10:42:17", "actor": "周芮", "action": "metadata.version.inspect", "target": "客户 / v1.6", "result": "成功"}, {"time": "2026-07-25 09:18:04", "actor": "陈微", "action": "authorization.snapshot.verify", "target": "权限快照", "result": "成功"}, {"time": "2026-07-25 08:55:31", "actor": "system", "action": "runtime.health.check", "target": "运行时节点", "result": "成功"}, {"time": "2026-07-24 18:10:09", "actor": "林川", "action": "organization.tree.inspect", "target": "组织架构", "result": "成功"}}
	actions := []string{"metadata.object.inspect", "authorization.role.inspect", "runtime.route.inspect", "audit.retention.inspect"}
	targets := []string{"对象目录", "角色定义", "运行路由", "审计保留策略"}
	actors := []string{"唐昕", "顾宁", "叶澜", "陆衡"}
	for index := 0; index < 20; index++ {
		events = append(events, map[string]string{"time": "2026-07-24 17:" + twoDigits(59-index) + ":00", "actor": actors[index%len(actors)], "action": actions[index%len(actions)], "target": targets[index%len(targets)], "result": "成功"})
	}
	return events
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}
