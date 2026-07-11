# 字典值定义

本文档统一管理所有设计文档中出现的枚举值、状态码、类型等字典值，避免在多个模块设计中重复定义。

## 提示词业务类型

### 提示词类型

| 值 | 说明 |
|---|---|
| PLAIN | 普通提示词，由用户编辑内容并发布 |
| COMPOSITE | 复合提示词，由表达式组合、计算或引用其他提示词生成 |

### 提示词分类

| 值 | 说明 |
|---|---|
| ENV | 环境配置类提示词 |
| DEV | 开发通用类提示词 |
| ROLE | 角色设定类提示词 |
| DB | 数据库类提示词 |
| FRONTEND | 前端类提示词 |
| BACKEND | 后端类提示词 |
| DOMAIN | 业务领域类提示词 |
| PROJECT | 项目规范类提示词 |
| PRODUCT | 产品设计类提示词 |
| JOB | 职位或岗位类提示词 |
| OPS | 运维类提示词 |

### 提示词制作指令

| 值 | 符号 | 说明 |
|---|---|---|
| SYNTHESIS | + | 汇总多个输入内容并形成综合结果 |
| KEYS |  | 提取关键词 |
| TAGS | # | 生成或附加标签 |
| LABELS |  | 生成标注、分类或结构化说明 |
| SUMMARY |  | 生成摘要 |
| MERGE | \| | 合并多个输入内容 |
| TITLE |  | 生成标题 |
| BULLETS |  | 生成要点列表 |
| CLEAN |  | 清洗噪声、格式和无效内容 |
| DEDUPE |  | 去除重复或近似重复内容 |
| TOPIC |  | 提炼主题 |
| OUTLINE |  | 生成大纲 |
| LEAD |  | 生成导语 |
| QUESTIONS |  | 生成问题清单 |
| EXTRACT |  | 抽取指定要素 |
| TEMPLATE |  | 按模板组织内容 |
| FORMAT |  | 排版与格式整理，统一结构与格式 |
| TRANSLATE |  | 将输入翻译为目标语言 |
| REDACT |  | 识别并遮蔽敏感信息 |
| REFINE |  | 提炼核心价值并精炼表达 |

### 版本选择规则

| 值 | 示例 | 说明 |
|---|---|---|
| EXACT | 1.2.3 | 精确匹配指定版本 |
| CARET | ^1.2.3 | 匹配同一主版本下的兼容版本 |
| TILDE | ~1.2.3 | 匹配同一主版本和次版本下的兼容版本 |
| GTE | >=1.2.3 | 匹配大于等于指定版本的版本 |
| LT | <2.0.0 | 匹配小于指定版本的版本 |
| LATEST | latest 或 * | 匹配最新可用发布版本 |

### 提示词下载渠道

| 值 | 说明 |
|---|---|
| URL | 通过 URL 查询或请求体提交表达式，查看、下载或实时生成提示词 |
| CLI | 通过命令行客户端搜索、查看、安装、升级和卸载提示词 |
| WEB | 通过用户端门户浏览、制作、发布和下载提示词 |
| MANAGER | 通过管理界面审核、上下架、分类和字典维护提示词 |

### 提示词计算时机

| 值 | 说明 |
|---|---|
| IMMEDIATE | 实时计算，依赖包变化后立即判断并重新计算受影响的提示词 |
| DEFERRED | 延时计算，在提示词被查看、下载、安装或引用时按需逐层计算 |

### 提示词缓存策略

| 值 | 说明 |
|---|---|
| NONE | 不使用缓存，每次请求都重新计算 |
| VERSIONED | 按表达式、依赖解析版本和参数缓存，适用于已发布提示词和复合提示词 |
| EPHEMERAL | 短期缓存实时生成结果，适用于重复请求较多但不需要长期保存的实时提示词 |

### 制作指令执行方式

| 值 | 说明 |
|---|---|
| LLM | 调用大语言模型生成结果，会消耗输入和输出 token |
| LOCAL | 使用本地确定性规则生成结果，通常不消耗模型 token |
| HYBRID | 先使用本地规则预处理，再调用大语言模型生成结果 |

### 前端应用类型

| 值 | 说明 |
|---|---|
| WEB | 用户端门户 |
| MANAGER | 管理界面 |
| CLI | 命令行客户端 |

### 页面操作类型

| 值 | 说明 |
|---|---|
| VIEW | 查看资源 |
| CREATE | 创建资源 |
| UPDATE | 修改资源 |
| DELETE | 删除或软删除资源 |
| PUBLISH | 发布提示词版本 |
| DEPRECATE | 废弃提示词版本 |
| GENERATE | 实时生成提示词 |
| CLEAR_CACHE | 清理缓存 |

## CLI 相关类型

### CLI 命令

| 值 | 说明 |
|---|---|
| ADD | 安装或添加提示词包到本地 repository 或目标项目 |
| REMOVE | 从本地 repository 或目标项目移除提示词包 |
| UPGRADE | 升级本地已安装的提示词包 |
| CREATE | 创建普通提示词或复合提示词包 |
| CACHE | 查看、校验或清理提示词计算缓存 |
| INFO | 查看提示词包、配置、依赖或 repository 信息 |
| CONFIG | 查看和修改 CLI 配置 |
| VERSION | 查看 CLI 版本和运行环境信息 |
| PUBLISH | 发布提示词包到远程 server |
| LOGIN | 登录远程 server |

## Server 数据库相关类型

### 账号状态

| 值 | 说明 |
|---|---|
| ACTIVE | 正常可用 |
| DISABLED | 已禁用，不能登录或访问受保护资源 |
| LOCKED | 因安全策略临时锁定 |

### 资源可见性

| 值 | 说明 |
|---|---|
| PUBLIC | 公开资源，可被匿名或所有用户查看 |
| PRIVATE | 私有资源，仅授权用户可访问 |
| INTERNAL | 内部资源，仅指定命名空间或组织内可访问 |

### 账号角色

| 值 | 说明 |
|---|---|
| ADMIN | 管理员，拥有系统管理权限 |
| USER | 普通用户，可发布和使用提示词 |

### 命名空间角色

| 值 | 说明 |
|---|---|
| OWNER | 拥有命名空间全部管理权限 |
| MAINTAINER | 可维护包、版本和成员协作内容 |
| MEMBER | 可查看和使用命名空间内授权资源 |
| VIEWER | 只读访问授权资源 |

### 包版本事件类型

| 值 | 说明 |
|---|---|
| CREATED | 版本草稿创建 |
| UPDATED | 版本元数据、内容或表达式更新 |
| PUBLISHED | 版本发布 |
| DEPRECATED | 版本废弃 |
| COMPUTED | 复合提示词计算完成 |
| CACHE_INVALIDATED | 相关计算缓存失效 |

### 审计操作类型

| 值 | 说明 |
|---|---|
| LOGIN | 用户登录 |
| LOGOUT | 用户退出或令牌失效 |
| CREATE_PACKAGE | 创建提示词包 |
| UPDATE_PACKAGE | 修改提示词包概要 |
| CREATE_VERSION | 创建提示词版本 |
| PUBLISH_VERSION | 发布提示词版本 |
| DEPRECATE_VERSION | 废弃提示词版本 |
| DELETE_RESOURCE | 删除或软删除资源 |
| UPDATE_CONFIG | 修改服务配置 |
| MANAGE_MEMBER | 管理命名空间成员 |
| GENERATE_PROMPT | 实时生成或计算提示词 |

### 操作结果

| 值 | 说明 |
|---|---|
| SUCCESS | 操作成功 |
| FAILURE | 操作失败 |
| DENIED | 因权限或策略被拒绝 |

### 风险类型

| 值 | 说明 |
|---|---|
| SENSITIVE_CONTENT | 内容疑似包含敏感信息 |
| POLICY_VIOLATION | 内容违反平台策略 |
| OUTDATED | 内容过期或不再推荐使用 |
| MALICIOUS | 内容存在恶意或高风险行为 |
| COPYRIGHT | 内容存在版权或授权风险 |

### 风险等级

| 值 | 说明 |
|---|---|
| LOW | 低风险，允许观察或提示 |
| MEDIUM | 中风险，需要处理或限制推荐 |
| HIGH | 高风险，需要限制访问或下架 |
| CRITICAL | 严重风险，需要立即阻断 |

### 风险处理状态

| 值 | 说明 |
|---|---|
| OPEN | 待处理 |
| REVIEWING | 审核中 |
| RESOLVED | 已处理 |
| IGNORED | 已确认无需处理 |

### 服务配置值类型

| 值 | 说明 |
|---|---|
| STRING | 字符串配置 |
| NUMBER | 数值配置 |
| BOOLEAN | 布尔配置 |
| JSON | 结构化 JSON 配置 |
| SECRET | 敏感配置，管理端不回显明文 |

### 管理资源类型

| 值 | 说明 |
|---|---|
| USER | 用户 |
| NAMESPACE | 命名空间 |
| PACKAGE | 提示词包 |
| PACKAGE_VERSION | 提示词包版本 |
| DICT_ITEM | 字典项 |
| SERVICE_CONFIG | 服务配置 |
| CACHE_ENTRY | 缓存项 |
| RISK_MARK | 风险标记 |
| AUDIT_LOG | 审计日志 |
| GENERATION_RECORD | 生成记录 |

### 服务健康状态

| 值 | 说明 |
|---|---|
| OK | 服务正常 |
| DEGRADED | 服务可用但部分依赖异常 |
| DOWN | 服务不可用 |

## SDK 相关类型

### SDK 输出形式

| 值 | 说明 |
|---|---|
| TEXT | 输出最终提示词文本 |
| MESSAGES | 输出面向对话模型的消息序列 |
| TRACE | 输出表达式执行追踪信息 |
| AST | 输出结构化表达结果 |

### SDK 错误码

| 值 | 说明 |
|---|---|
| SDK_PARSE_FAILED | 表达式词法或语法解析失败 |
| SDK_SEMANTIC_INVALID | 表达式语义校验失败 |
| SDK_DEPENDENCY_LOAD_FAILED | 调用方提供的内容加载能力无法加载依赖内容 |
| SDK_DEPENDENCY_CYCLE | 表达式依赖链存在循环引用 |
| SDK_CONTEXT_MISSING | 运行所需上下文变量缺失 |
| SDK_INSTRUCTION_FAILED | 制作指令执行失败 |
| SDK_RUNTIME_BUDGET_EXCEEDED | 表达式运行超过深度、依赖数量或 token 预算限制 |
| SDK_CACHE_UNAVAILABLE | 调用方提供的缓存能力不可用或缓存操作失败 |

## Registry 相关类型

### Registry 错误码

| 值 | 说明 |
|---|---|
| INVALID_REQUEST | 请求格式或参数不合法 |
| UNAUTHORIZED | 未认证或认证已失效 |
| FORBIDDEN | 已认证但无权访问目标资源 |
| PACKAGE_NOT_FOUND | 提示词包不存在 |
| VERSION_NOT_FOUND | 指定版本不存在或版本规则无法匹配 |
| VERSION_CONFLICT | 发布版本已存在，不能覆盖 |
| INVALID_PACKAGE | package 属性不合法 |
| INVALID_EXPRESSION | expression 语法或语义不合法 |
| DEPENDENCY_NOT_FOUND | 依赖包或依赖版本不存在 |
| DEPENDENCY_CYCLE | 依赖图存在循环引用 |
| GENERATION_FAILED | 提示词计算或 LLM 生成失败 |
| CACHE_UNAVAILABLE | 缓存不可用或缓存操作失败 |
| RATE_LIMITED | 请求超过频率限制 |
| SERVER_ERROR | 服务端内部错误 |

## 提示词结构类型

### 指令类型

| 值 | 说明 |
|---|---|
| TEXT | 文本指令，输出固定文本或表达式结果 |
| VARIABLE | 变量指令，声明可被表达式引用的输入变量 |
| BLOCK | 块指令，组织一组子指令 |
| REFERENCE | 引用指令，复用其他提示词或指令片段 |
| MESSAGE | 消息指令，表示一条完整的对话消息 |
| TOOL_CALL | 工具调用指令，描述需要调用的外部工具 |
| TOOL_RESULT | 工具结果指令，承载工具调用的返回结果 |

### 消息角色

| 值 | 说明 |
|---|---|
| SYSTEM | 系统级角色，定义全局行为与约束 |
| USER | 用户角色，表示用户输入 |
| ASSISTANT | 助手角色，表示模型回复 |
| TOOL | 工具角色，表示工具返回的内容 |

### 值类型

| 值 | 说明 |
|---|---|
| STRING | 字符串 |
| NUMBER | 数值 |
| BOOLEAN | 布尔值 |
| LIST | 列表 |
| MAP | 键值映射 |
| NULL | 空值 |

### 表达式用途

| 值 | 说明 |
|---|---|
| VALUE | 计算指令输出内容 |
| ENABLED | 判断指令是否参与渲染 |
| REPEAT | 计算指令重复渲染的数据来源 |
| PARAMETER | 计算引用或工具调用参数 |

### 错误处理策略

| 值 | 说明 |
|---|---|
| STRICT | 严格模式，表达式求值失败时终止渲染 |
| LENIENT | 宽容模式，表达式求值失败时使用空值或默认值继续渲染，并记录警告 |

## 存储相关类型

### 存储格式

| 值 | 说明 |
|---|---|
| YAML | YAML 格式，便于人工阅读和编辑 |
| JSON | JSON 格式，便于程序解析和传输 |

### 版本状态

| 值 | 说明 |
|---|---|
| DRAFT | 草稿状态，未发布 |
| PUBLISHED | 已发布状态，可搜索、下载、安装和引用 |
| DEPRECATED | 已废弃，不建议继续使用 |

### 引用类型

| 值 | 说明 |
|---|---|
| EMBED | 内嵌引用，将目标内容展开到当前位置 |
| LINK | 链接引用，仅在元数据中记录依赖关系 |
