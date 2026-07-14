package service

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hub-server/internal/model"
)

type DictService struct {
	db *gorm.DB
}

func NewDictService(db *gorm.DB) *DictService {
	return &DictService{db: db}
}

func (s *DictService) SeedDefaults() error {
	items := DefaultDictItems()
	for _, item := range items {
		if err := s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "group"}, {Name: "value"}}, DoUpdates: clause.AssignmentColumns([]string{"label", "description", "sort_order", "enabled", "is_system"})}).Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *DictService) All(enabledOnly bool) (map[string][]model.DictItem, error) {
	var items []model.DictItem
	query := s.db.Order("`group` asc, sort_order asc, value asc")
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	out := map[string][]model.DictItem{}
	for _, item := range items {
		out[item.Group] = append(out[item.Group], item)
	}
	return out, nil
}

func (s *DictService) Group(group string, enabledOnly bool) ([]model.DictItem, error) {
	var items []model.DictItem
	query := s.db.Where("`group` = ?", group).Order("sort_order asc, value asc")
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	return items, query.Find(&items).Error
}

func DefaultDictItems() []model.DictItem {
	defs := []struct{ group, value, label, desc string }{
		{"prompt_type", "PLAIN", "普通提示词", "由用户编辑内容并发布"},
		{"prompt_type", "COMPOSITE", "复合提示词", "由表达式组合生成"},
		{"prompt_category", "ENV", "环境", "环境配置类提示词"},
		{"prompt_category", "DEV", "开发", "开发通用类提示词"},
		{"prompt_category", "ROLE", "角色", "角色设定类提示词"},
		{"prompt_category", "DB", "数据库", "数据库类提示词"},
		{"prompt_category", "FRONTEND", "前端", "前端类提示词"},
		{"prompt_category", "BACKEND", "后端", "后端类提示词"},
		{"prompt_category", "DOMAIN", "领域", "业务领域类提示词"},
		{"prompt_category", "PROJECT", "项目", "项目规范类提示词"},
		{"prompt_category", "PRODUCT", "产品", "产品设计类提示词"},
		{"prompt_category", "JOB", "职位", "职位或岗位类提示词"},
		{"prompt_category", "OPS", "运维", "运维类提示词"},
		{"version_status", "DRAFT", "草稿", "未发布"},
		{"version_status", "PUBLISHED", "已发布", "可搜索、下载、安装和引用"},
		{"version_status", "DEPRECATED", "已废弃", "不建议继续使用"},
		{"version_rule", "LATEST", "最新", "匹配最新可用发布版本"},
		{"version_rule", "EXACT", "精确", "精确匹配指定版本，如 1.2.3"},
		{"version_rule", "CARET", "主版本兼容", "匹配同一主版本下兼容版本，如 ^1.2.3"},
		{"version_rule", "TILDE", "次版本兼容", "匹配同一主次版本下兼容版本，如 ~1.2.3"},
		{"version_rule", "GTE", "大于等于", "匹配大于等于指定版本，如 >=1.2.3"},
		{"version_rule", "LT", "小于", "匹配小于指定版本，如 <2.0.0"},
		{"visibility", "PUBLIC", "公开", "公开资源"},
		{"visibility", "PRIVATE", "私有", "私有资源"},
		{"visibility", "INTERNAL", "内部", "内部资源"},
		{"account_status", "ACTIVE", "正常", "正常可用"},
		{"account_status", "DISABLED", "禁用", "不能登录或访问"},
		{"account_status", "LOCKED", "锁定", "临时锁定"},
		{"manager_role", "ADMIN", "管理员", "管理端管理员，拥有全部管理权限"},
		{"account_source", "LOCAL", "本地", "本地账号，使用密码认证"},
		{"account_source", "LDAP", "LDAP", "LDAP 目录同步账号"},
		{"account_source", "WEB", "Web", "Web 自助注册账号（预留）"},
		{"namespace_role", "OWNER", "所有者", "拥有全部管理权限"},
		{"namespace_role", "MAINTAINER", "维护者", "可维护包和成员"},
		{"namespace_role", "MEMBER", "成员", "可使用授权资源"},
		{"namespace_role", "VIEWER", "只读", "只读访问"},
		{"cache_strategy", "NONE", "不缓存", "不使用缓存"},
		{"cache_strategy", "VERSIONED", "版本缓存", "按表达式和依赖版本缓存"},
		{"cache_strategy", "EPHEMERAL", "短期缓存", "实时生成短期缓存"},
		{"error_strategy", "STRICT", "严格", "求值失败时终止"},
		{"error_strategy", "LENIENT", "宽容", "求值失败时使用默认值继续"},
		{"prompt_instruction", "SYNTHESIS", "汇总", "综合多个输入为一个结果，运算符 +"},
		{"prompt_instruction", "KEYS", "提取关键词", "提取核心关键词"},
		{"prompt_instruction", "TAGS", "标签", "生成或附加标签，运算符 #"},
		{"prompt_instruction", "LABELS", "标注", "生成标注或结构化说明"},
		{"prompt_instruction", "SUMMARY", "摘要", "压缩并保留核心含义"},
		{"prompt_instruction", "MERGE", "合并", "按顺序合并输入，运算符 |"},
		{"prompt_instruction", "TITLE", "标题", "生成标题"},
		{"prompt_instruction", "BULLETS", "要点", "整理为要点列表"},
		{"prompt_instruction", "CLEAN", "清洗", "移除噪声与无效内容"},
		{"prompt_instruction", "DEDUPE", "去重", "去除重复或近似重复"},
		{"prompt_instruction", "TOPIC", "主题", "提炼主题"},
		{"prompt_instruction", "OUTLINE", "大纲", "生成层级大纲"},
		{"prompt_instruction", "LEAD", "导语", "生成开头说明"},
		{"prompt_instruction", "QUESTIONS", "问题", "生成问题清单"},
		{"prompt_instruction", "EXTRACT", "要素抽取", "抽取指定要素"},
		{"prompt_instruction", "TEMPLATE", "模板", "按模板组织内容"},
		{"prompt_instruction", "FORMAT", "内容排版", "排版与格式整理，统一结构与格式"},
		{"prompt_instruction", "TRANSLATE", "翻译", "将输入翻译为目标语言"},
		{"prompt_instruction", "REDACT", "脱敏", "识别并遮蔽敏感信息"},
		{"prompt_instruction", "REFINE", "提炼", "提炼核心价值并精炼表达"},
		{"risk_type", "SENSITIVE_CONTENT", "敏感内容", "疑似包含敏感信息"},
		{"risk_type", "POLICY_VIOLATION", "策略违规", "违反平台策略"},
		{"risk_type", "OUTDATED", "过期", "内容过期"},
		{"risk_type", "MALICIOUS", "恶意", "存在恶意风险"},
		{"risk_type", "COPYRIGHT", "版权", "存在版权风险"},
		{"risk_severity", "LOW", "低", "低风险"},
		{"risk_severity", "MEDIUM", "中", "中风险"},
		{"risk_severity", "HIGH", "高", "高风险"},
		{"risk_severity", "CRITICAL", "严重", "严重风险"},
		{"risk_status", "OPEN", "待处理", "待处理"},
		{"risk_status", "REVIEWING", "审核中", "审核中"},
		{"risk_status", "RESOLVED", "已处理", "已处理"},
		{"risk_status", "IGNORED", "已忽略", "无需处理"},
		{"operation_result", "SUCCESS", "成功", "操作成功"},
		{"operation_result", "FAILURE", "失败", "操作失败"},
		{"operation_result", "DENIED", "拒绝", "权限或策略拒绝"},
	}
	items := make([]model.DictItem, 0, len(defs))
	for i, def := range defs {
		items = append(items, model.DictItem{Group: def.group, Value: def.value, Label: def.label, Description: def.desc, SortOrder: i + 1, Enabled: true, System: true})
	}
	return items
}
