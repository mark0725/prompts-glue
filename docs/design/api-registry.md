# Registry API 设计

## 概述

Registry API 是 prompts-glue 的远程 server 接口，面向 `glue` CLI、用户端门户和管理界面提供提示词包注册、搜索、查看、下载、发布、版本管理、依赖解析和实时生成能力。

Registry 负责维护已发布提示词包及其版本、package 属性、内容、表达式、依赖关系、发布审计和访问控制。CLI 在非本地模式下通过 Registry API 完成远程包操作。

## 设计目标

- 为提示词包提供统一的远程发布、发现、下载和升级入口。
- 支持普通提示词和复合提示词的版本化管理。
- 支持根据 expression 解析依赖并生成复合提示词结果。
- 支持实时提示词生成，配合缓存减少重复 token 消耗。
- 支持 CLI、Web、Manager 使用同一套 API 语义。
- 返回结构稳定，便于命令行展示、前端渲染和自动化处理。

## API 基础约定

Registry API 遵循 [api.md](api.md) 中定义的通用 API 基础约定，包括数据格式、认证方式、统一响应结构、错误对象、分页结构和安全要求。

| 项目 | 说明 |
|---|---|
| API 前缀 | `/api/registry` |
| 包内容格式 | Markdown 内容体与 package 属性分离传输 |

公开搜索、查看公开包和下载公开包可匿名访问；发布包、废弃版本、查看私有包和缓存管理必须按权限认证。

## 数据对象

### Package 属性

Package 属性与 [prompts.md](prompts.md) 中的“Package 属性”保持一致。

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 是 | 包名 |
| version | 字符串 | 是 | 版本号 |
| description | 字符串 | 是 | 提示词用途说明 |
| status | 版本状态 | 是 | 取值见 dicts.md |
| category | 提示词分类 | 是 | 取值见 dicts.md |
| tags | 字符串列表 | 否 | 标签 |
| author | 字符串 | 否 | 作者或维护者 |
| repository | 字符串 | 否 | 源码仓库或维护仓库地址 |
| homepage | 字符串 | 否 | 介绍页面、文档页面或项目主页地址 |
| license | 字符串 | 否 | 授权协议 |
| type | 提示词类型 | 是 | 取值见 dicts.md |
| visibility | 资源可见性 | 否 | 取值见 dicts.md；发布时设置，未提供默认公开 |
| expression | 字符串 | 条件必填 | 复合提示词计算表达式 |
| dependencies | 依赖列表 | 否 | 系统根据 expression 自动计算动态生成 |
| created_at | UTC 时间 | 是 | 创建时间 |

### Package 内容

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| package | Package 属性 | 是 | 提示词包元数据 |
| content | 字符串 | 是 | Markdown 内容体 |
| readme | 字符串 | 否 | 包说明内容 |
| checksum | 字符串 | 否 | 内容校验摘要 |

### 依赖对象

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 是 | 依赖包名 |
| version_rule | 版本选择规则 | 否 | expression 中声明的版本规则 |
| resolved_version | 字符串 | 是 | 实际解析到的版本 |
| optional | 布尔值 | 否 | 是否允许缺失 |
| affects_compute | 布尔值 | 否 | 依赖变化是否影响重新计算 |

### 计算记录

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| expression | 字符串 | 是 | 本次计算使用的表达式 |
| dependencies | 依赖列表 | 是 | 实际解析到的依赖 |
| cache_hit | 布尔值 | 是 | 是否命中缓存 |
| cache_key | 字符串 | 否 | 缓存键摘要 |
| token_usage | Token 用量摘要 | 否 | 输入、输出和总 token |
| computed_at | UTC 时间 | 是 | 计算完成时间 |

## 接口清单

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/auth/login` | 登录并获取访问凭据 |
| GET | `/auth/me` | 查看当前登录身份 |
| GET | `/packages` | 搜索提示词包 |
| GET | `/packages/{name}` | 查看包概要和版本列表 |
| GET | `/packages/{name}/versions/{version}` | 查看指定版本详情 |
| GET | `/packages/{name}/versions/{version}/content` | 下载指定版本内容 |
| POST | `/packages` | 发布新提示词包版本 |
| POST | `/packages/{name}/versions/{version}/deprecate` | 废弃指定版本 |
| POST | `/resolve` | 解析包名和版本规则 |
| POST | `/dependencies/resolve` | 解析 expression 依赖图 |
| POST | `/generate` | 根据 expression 实时生成提示词 |
| GET | `/cache` | 查看服务端计算缓存信息 |
| DELETE | `/cache` | 清理服务端计算缓存 |
| GET | `/health` | 查看 Registry 服务状态 |

## 认证接口

### 登录

| 项目 | 说明 |
|---|---|
| 方法 | POST |
| 路径 | `/api/registry/auth/login` |
| 认证 | 不需要 |
| 用途 | 获取访问 Registry 的认证凭据 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| username | 字符串 | 是 | 用户名、邮箱或账号标识 |
| password | 字符串 | 条件必填 | 密码凭据 |
| token | 字符串 | 条件必填 | 外部访问令牌 |

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| access_token | 字符串 | 是 | 访问凭据 |
| expires_at | UTC 时间 | 否 | 过期时间 |
| user | 用户对象 | 是 | 当前用户信息 |

### 当前身份

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/auth/me` |
| 认证 | 需要 |
| 用途 | 返回当前登录用户、权限和默认命名空间 |

## 包查询接口

### 搜索包

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/packages` |
| 认证 | 公开包可匿名，私有包需要认证 |
| 用途 | 搜索、筛选和分页浏览提示词包 |

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| q | 字符串 | 否 | 按包名、描述、标签搜索 |
| category | 提示词分类 | 否 | 按分类筛选 |
| tag | 字符串 | 否 | 按标签筛选 |
| author | 字符串 | 否 | 按作者筛选 |
| status | 版本状态 | 否 | 按版本状态筛选 |
| type | 提示词类型 | 否 | 按提示词类型筛选 |
| page | 数值 | 否 | 页码 |
| page_size | 数值 | 否 | 每页数量 |

响应数据为分页结构，`items` 中每项包含包名、描述、最新版本、状态、分类、标签、作者和更新时间。

### 查看包概要

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/packages/{name}` |
| 认证 | 根据包可见性决定 |
| 用途 | 查看包概要和版本列表 |

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 是 | 包名 |
| description | 字符串 | 是 | 包说明 |
| latest_version | 字符串 | 否 | 最新可用版本 |
| versions | 版本概要列表 | 是 | 版本列表 |
| tags | 字符串列表 | 否 | 标签 |
| category | 提示词分类 | 是 | 分类 |

### 查看版本详情

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/packages/{name}/versions/{version}` |
| 认证 | 根据包可见性决定 |
| 用途 | 查看指定版本 package 属性、依赖和计算记录 |

响应数据包含 Package 属性、依赖列表、计算记录、发布者和发布时间。

### 下载版本内容

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/packages/{name}/versions/{version}/content` |
| 认证 | 根据包可见性决定 |
| 用途 | 下载指定版本提示词内容 |

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| format | 字符串 | 否 | 响应形态，见下表；默认 `json` |
| view | 布尔值 | 否 | `true` 时仅预览，不累计下载统计 |
| include_dependencies | 布尔值 | 否 | 是否同时返回依赖包信息 |

`format` 取值：

| 值 | 响应 | 说明 |
|---|---|---|
| `json` 或未提供 | 统一 API 外壳 + Package 内容对象 | 默认；CLI `add`/`upgrade` 与 Web 预览加载使用 |
| `raw` | 纯文本正文 | `Content-Type: text/markdown; charset=utf-8`，仅返回 Markdown 提示内容体，不含 package 元数据与 API 外壳；Web「提示内容 URL」使用 |

响应数据（`format=json`）为 Package 内容。CLI 的 `add` 和 `upgrade` 命令使用该接口获取包内容。

`format=raw` 时：

- 成功直接写出正文，不使用统一 JSON 外壳。
- 失败仍返回统一错误 JSON，便于客户端识别。
- 可与 `view=true` 组合：预览用 URL 不累计下载；curl 下载包使用默认 `format` 并计次。

## 发布接口

### 发布包版本

| 项目 | 说明 |
|---|---|
| 方法 | POST |
| 路径 | `/api/registry/packages` |
| 认证 | 需要 |
| 用途 | 发布一个新的提示词包版本 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| package | Package 属性 | 是 | 提示词包属性 |
| content | 字符串 | 是 | Markdown 内容体 |
| compute | 布尔值 | 否 | 是否在发布前计算复合提示词 |
| dry_run | 布尔值 | 否 | 只校验不发布 |

发布校验：

- 包名必须符合包命名规则。
- 版本号必须符合语义化版本规则。
- `type` 为 composite 时必须提供 expression。
- dependencies 由 Registry 根据 expression 解析生成，不信任客户端手工传入结果。
- 相同包名和版本已发布时不得覆盖。
- 发布者必须具备目标包或命名空间的维护权限。

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| package | Package 属性 | 是 | 发布后的 package 属性 |
| dependencies | 依赖列表 | 是 | Registry 解析后的依赖 |
| calculation | 计算记录 | 否 | 复合提示词计算记录 |
| warnings | 字符串列表 | 否 | 非阻断性校验提示 |

### 废弃版本

| 项目 | 说明 |
|---|---|
| 方法 | POST |
| 路径 | `/api/registry/packages/{name}/versions/{version}/deprecate` |
| 认证 | 需要维护权限 |
| 用途 | 将指定版本标记为废弃 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| reason | 字符串 | 是 | 废弃原因 |
| replacement | 字符串 | 否 | 建议替代包或版本 |

废弃不会删除历史版本，也不破坏依赖该精确版本的可复现性。

## 解析接口

### 解析版本规则

| 项目 | 说明 |
|---|---|
| 方法 | POST |
| 路径 | `/api/registry/resolve` |
| 认证 | 根据包可见性决定 |
| 用途 | 根据包名和版本规则解析实际版本 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 是 | 包名 |
| version_rule | 版本选择规则 | 否 | 版本选择规则，未提供时使用 latest |
| include_deprecated | 布尔值 | 否 | 是否允许匹配已废弃版本 |

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 是 | 包名 |
| version_rule | 版本选择规则 | 是 | 使用的版本规则 |
| resolved_version | 字符串 | 是 | 实际解析到的版本 |
| status | 版本状态 | 是 | 解析版本状态 |

### 解析依赖图

| 项目 | 说明 |
|---|---|
| 方法 | POST |
| 路径 | `/api/registry/dependencies/resolve` |
| 认证 | 根据依赖包可见性决定 |
| 用途 | 解析 expression 中的直接和间接依赖 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| expression | 字符串 | 是 | 提示词计算表达式 |
| include_transitive | 布尔值 | 否 | 是否返回间接依赖 |
| error_strategy | 错误处理策略 | 否 | 依赖解析失败时的处理策略 |

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| dependencies | 依赖列表 | 是 | 解析到的依赖 |
| graph | 依赖图 | 否 | 包之间的依赖关系 |
| warnings | 字符串列表 | 否 | 非阻断性问题 |

解析依赖图时必须检测循环引用，发现循环时返回依赖循环错误。

## 实时生成接口

### 根据 expression 生成提示词

| 项目 | 说明 |
|---|---|
| 方法 | POST |
| 路径 | `/api/registry/generate` |
| 认证 | 根据服务策略决定 |
| 用途 | 提交 expression 并实时生成提示词内容 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| expression | 字符串 | 是 | 提示词计算表达式 |
| inputs | 映射 | 否 | 调用方输入参数 |
| cache_strategy | 提示词缓存策略 | 否 | 本次生成的缓存策略 |
| error_strategy | 错误处理策略 | 否 | 表达式或依赖失败时的处理策略 |
| save_as_package | 布尔值 | 否 | 是否将结果保存为提示词包草稿 |
| package | Package 属性 | 条件必填 | 保存为包时使用的 package 属性 |

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| content | 字符串 | 是 | 生成后的提示词内容 |
| dependencies | 依赖列表 | 是 | 实际解析到的依赖 |
| calculation | 计算记录 | 是 | 计算、缓存和 token 记录 |
| package | Package 属性 | 否 | 保存为包时返回的 package 属性 |

实时生成应优先使用短期缓存。包含敏感内容、超大文本或不可缓存标记的请求不应写入共享缓存。

## 缓存接口

### 查看缓存信息

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/cache` |
| 认证 | 需要管理权限或维护权限 |
| 用途 | 查看服务端计算缓存统计和缓存项 |

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 否 | 按包名筛选 |
| expression_hash | 字符串 | 否 | 按表达式摘要筛选 |
| page | 数值 | 否 | 页码 |
| page_size | 数值 | 否 | 每页数量 |

### 清理缓存

| 项目 | 说明 |
|---|---|
| 方法 | DELETE |
| 路径 | `/api/registry/cache` |
| 认证 | 需要管理权限或维护权限 |
| 用途 | 清理服务端计算缓存 |

请求字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| name | 字符串 | 否 | 清理指定包相关缓存 |
| expression_hash | 字符串 | 否 | 清理指定表达式缓存 |
| expired_only | 布尔值 | 否 | 仅清理过期缓存 |

## 健康检查接口

### 服务状态

| 项目 | 说明 |
|---|---|
| 方法 | GET |
| 路径 | `/api/registry/health` |
| 认证 | 不需要 |
| 用途 | 查看 Registry 服务状态 |

响应数据：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| status | 字符串 | 是 | 服务状态 |
| version | 字符串 | 是 | Registry 服务版本 |
| time | UTC 时间 | 是 | 服务端时间 |

## CLI 对应关系

| CLI 命令 | Registry API |
|---|---|
| add | 搜索包、查看版本详情、下载版本内容、解析版本规则 |
| upgrade | 搜索包、解析版本规则、下载版本内容 |
| create | 本地创建时不访问 Registry；非本地且保存远程草稿时使用发布或生成接口 |
| cache | 查看缓存信息、清理缓存 |
| info | 查看包概要、查看版本详情、当前身份 |
| publish | 发布包版本、解析依赖图 |
| login | 登录、当前身份 |
| version | 服务状态 |

## 安全要求

- 发布、废弃、私有包访问和缓存管理必须校验身份与权限。
- Registry 不信任客户端传入的 dependencies，应根据 expression 重新解析。
- 认证凭据不得出现在 URL、日志、package 内容、计算缓存或错误详情中。
- 实时生成接口应限制请求大小、调用频率和最大依赖深度。
- LLM 调用产生的 token 用量应记录到计算记录，但不应泄露其他用户输入。
- 包内容下载应返回校验摘要，CLI 可用于验证本地 repository 内容完整性。

## 错误处理

常见错误码见 [dicts.md](dicts.md) 中的“Registry 错误码”定义。

| 场景 | 处理要求 |
|---|---|
| 包不存在 | 返回包不存在错误，附带包名 |
| 版本不存在 | 返回版本不存在错误，附带版本规则和可用版本摘要 |
| 版本冲突 | 发布时返回版本冲突错误，不覆盖已有版本 |
| 依赖解析失败 | 返回失败依赖、版本规则和失败原因 |
| 依赖循环 | 返回循环路径，终止解析或生成 |
| 权限不足 | 返回权限错误，不暴露私有资源详情 |
| 表达式错误 | 返回表达式错误位置和说明 |
| 生成失败 | 返回生成失败原因和可重试提示 |
