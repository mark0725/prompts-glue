# prompts-glue

prompts builder for claude-code, codex, opencode

## 系统概述

各种 Agent 客户端有大量提示词，每个客户端或项目都需要重新编写类似提示词，重复、碎片化、浪费时间。prompts-glue 用于集中制作、发布、下载和管理 Agent 提示词。

系统包含后端服务、命令行客户端、用户端门户、管理界面和提示词表达式 SDK：

| 模块 | 目录 | 说明 |
|---|---|---|
| 后端服务 | `hub-server/` | 提供 Registry API、字典 API、管理 API、认证、缓存、审计和数据库能力 |
| 命令行客户端 | `cli/` | 提供 `glue` 命令，用于本地和远程管理提示词包 |
| 用户端门户 | `web/` | 用户搜索、浏览、查看、制作、实时生成和发布提示词 |
| 管理界面 | `manager/` | 系统运营、用户、字典、配置、风险、审计和缓存管理 |
| 提示词 SDK | `prompts-sdk/` | 提示词表达式解析、校验、依赖抽取和运行 |

详细设计见 `docs/design/README.md`。

## 提示词制作

- 普通提示词：在用户界面或 CLI 中编辑并发布，发布后可搜索和下载。
- 复合提示词：通过表达式组合已有提示词、文本片段和制作指令。
- 实时提示词：通过 API 提交表达式并即时返回生成结果。
- 提示词制作指令包括汇总、提取关键词、标签、标注、摘要、合并、标题、要点、清洗、去重、主题、大纲、导语、问题、要素抽取、模板、内容排版、翻译、脱敏和提炼等。
- 提示词分类、类型、状态和缓存策略等编码由后端字典统一管理。

提示词规则详细设计见 `docs/design/prompts.md`。

## 提示词存储

提示词包使用 Markdown 文件保存内容，文件头使用 YAML frontmatter 描述 package 属性。服务端保存权威数据，CLI 本地 repository 用于离线工作和本地缓存。

包名规则：

- 使用小写字母、数字、连字符或下划线，推荐连字符。
- 不允许包含点号、空格、特殊字符或大写字母。
- 不允许以点号或下划线开头。
- 长度不超过 214 个字符。
- 组织或作用域包使用 `@scope/name` 格式。

## 提示词分发

- Web：通过用户端门户搜索、浏览、查看、制作和发布提示词。
- API：通过 Registry API 搜索、下载、发布、解析依赖和实时生成提示词。
- CLI：通过 `glue` 命令搜索、查看、安装、升级、发布和登录。

## 命令行

命令行工具名称为 `glue`。

```bash
glue [options] <add|remove|upgrade|create|content|cache|info|config|version|publish|login>
```

常用配置：

- `--local`：本地操作模式，不连接远程 server。
- `--llm-model`：指定提示词制作和计算使用的语言模型。
- `--llm-api-base`：指定语言模型服务访问入口。
- `--llm-api-key`：指定语言模型服务 API Key。
- `--llm-proxy`：指定访问语言模型服务时使用的代理。

默认配置目录为 `~/.glue/{config.yaml,repository}`，可通过 `GLUE_HOME` 调整。配置优先级为：命令行选项 > 环境变量 > config.yaml。

命令行详细设计见 `docs/design/app-cli.md`。

## Agent 环境中使用 prompts-glue

### claude

```bash
glue content test > .claude/prompts/test.md
```

在 `CLAUDE.md` 文件中加入：

```md
@.claude/prompts/test.md
```

### codex

待补充。

### opencode

待补充。

## 编译

### 一键构建（前端 + 后端）

```bash
./build.sh
```

脚本会构建 `web`、`manager`，把产物复制到 `hub-server/web`、`hub-server/manager`，再编译出 `bin/hub-server`。跳过依赖安装可设置 `SKIP_NPM_INSTALL=1`。

本地运行（静态目录按当前工作目录解析，需在 `hub-server/` 下启动）：

```bash
cd hub-server
../bin/hub-server --config config.example.yaml
```

默认访问：API `http://localhost:8080`、Web `http://localhost:8081`、Manager `http://localhost:8082`。仅启动 API 时追加 `--web-enable=false --manager-enable=false`。

### 后端服务

```bash
cd hub-server
GOPROXY="https://goproxy.cn,direct" go test ./...
GOPROXY="https://goproxy.cn,direct" go build ./...
```

后端默认监听 API `:8080`、Web `:8081`、Manager `:8082`；Web 和 Manager 端口从当前工作目录下的 `web/`、`manager/` 读取构建产物，可通过 `web.dir` / `manager.dir` 调整。

### 提示词 SDK

```bash
cd prompts-sdk
GOPROXY="https://goproxy.cn,direct" go test ./...
```

### 命令行客户端

```bash
cd cli
GOPROXY="https://goproxy.cn,direct" go test ./...
GOPROXY="https://goproxy.cn,direct" go build -o glue .
```

### 用户端门户

```bash
cd web
npm install
npm run build
```

构建产物在 `web/dist/`；根目录 `./build.sh` 会将其复制到 `hub-server/web/`。

### 管理界面

```bash
cd manager
npm install
npm run build
```

构建产物在 `manager/dist/`；根目录 `./build.sh` 会将其复制到 `hub-server/manager/`。

## 安装

后端服务可通过本地二进制或 Docker 部署。CLI 可通过编译生成的 `glue` 二进制使用。Web 和 Manager 由 hub-server 直接托管，不再需要 nginx。

服务配置示例见各模块的示例配置文件。

### Docker Compose 部署

项目根目录提供 `docker-compose.yml`，使用单个 hub-server 容器同时提供 API、用户端门户和管理界面，无需 nginx。MySQL 数据库独立部署在外部实例 `10.2.20.161:3306`，库名 `glue_db`，不纳入本编排。

```bash
# 设置外部 MySQL 账号（必填，未设置时 compose 会报错终止）
export GLUE_DB_USER=<账号>
export GLUE_DB_PASSWORD=<密码>   # 含特殊字符时按 net/url 规则转义

# 构建并启动（首次会自动构建前端和后端镜像）
docker compose up -d --build

# 查看日志
docker compose logs -f glue-hub-server

# 停止并清理
docker compose down
```

启动后访问：

| 服务 | 地址 |
|---|---|
| 后端 API | http://localhost:10800 |
| 用户端门户 web | http://localhost:10801 |
| 管理界面 manager | http://localhost:10802 |

如需拆分部署，可用同一镜像启动多个容器，并通过 `HUB_WEB__ENABLE=false` 或 `HUB_MANAGER__ENABLE=false` 关闭不需要的前端端口；各容器连接同一数据库。

生产部署前请通过环境变量覆盖默认凭据，例如：

```bash
GLUE_DB_USER=<账号> \
GLUE_DB_PASSWORD=<密码> \
HUB_AUTH__ADMIN_USERNAME=admin \
HUB_AUTH__ADMIN_PASSWORD=<强密码> \
docker compose up -d --build
```

hub-server 配置规范：配置文件支持 YAML/JSON/TOML（按扩展名识别），所有参数均可通过环境变量和命令行参数配置，优先级为 **命令行参数 > 环境变量 > 配置文件 > 代码默认值**。环境变量以 `HUB_` 开头，`__` 表示层级（如 `HUB_LLM__MODEL` 对应 `llm.model`）；命令行用横杠表示层级（如 `--llm-model`）。

常用配置环境变量：

| 变量　　　　　　　　　　　　　　　　　　　　　　　　　　　　| 说明　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　|
| -------------------------------------------------------------| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `GLUE_DB_USER`　　　　　　　　　　　　　　　　　　　　　　　| 外部 MySQL 账号（必填）　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　 |
| `GLUE_DB_PASSWORD`　　　　　　　　　　　　　　　　　　　　　| 外部 MySQL 密码（必填，含特殊字符需转义）　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　 |
| `HUB_AUTH__ADMIN_USERNAME`　　　　　　　　　　　　　　　　　| 初始化管理员用户名，默认 `admin`　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　|
| `HUB_AUTH__ADMIN_PASSWORD`　　　　　　　　　　　　　　　　　| 初始化管理员密码，生产必须修改　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　|
| `HUB_LOGGING__LEVEL`　　　　　　　　　　　　　　　　　　　　| 日志级别：`debug`/`info`/`warn`/`error`　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　 |
| `HUB_WEB__ENABLE` / `HUB_WEB__ADDRESS` / `HUB_WEB__DIR` | 用户端门户端口开关、监听地址（默认 `:8081`）与静态目录（默认 `web`） |
| `HUB_MANAGER__ENABLE` / `HUB_MANAGER__ADDRESS` / `HUB_MANAGER__DIR` | 管理界面端口开关、监听地址（默认 `:8082`）与静态目录（默认 `manager`） |
| `HUB_LLM__MODEL` / `HUB_LLM__API_BASE` / `HUB_LLM__API_KEY` | 实时生成与复合提示词计算使用的语言模型配置　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　|
| `HUB_LDAP__ENABLE`　　　　　　　　　　　　　　　　　　　　　| 是否启用 LDAP 用户同步与登录验证，默认 `false`　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　|
| `HUB_LDAP__HOST` / `HUB_LDAP__PORT`　　　　　　　　　　　　 | LDAP 服务地址与端口　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　 |
| `HUB_LDAP__BIND_DN` / `HUB_LDAP__BIND_PASSWORD`　　　　　　 | 同步用服务账号 DN 与密码　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　|
| `HUB_LDAP__SEARCH_DN`　　　　　　　　　　　　　　　　　　　 | 用户搜索基准 DN　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　 |
| `HUB_LDAP__SYNC_TARGETS__0`　　　　　　　　　　　　　　　　 | 同步目标数组首元素，可选 `manager`、`web`（也接受带尾 `__` 的 `__0__` 写法）。环境变量数组会整体覆盖默认值，要同时同步 manager 与 web 需设置 `HUB_LDAP__SYNC_TARGETS__0=manager` 与 `HUB_LDAP__SYNC_TARGETS__1=web` |
| `HUB_LDAP__SYNC_INTERVAL`　　　　　　　　　　　　　　　　　 | 周期同步间隔，如 `30m`；不大于 0 时不启动周期同步　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　　 |

| `HUB_LDAP__FILTER` | 账号名(uid)正则，匹配的才同步；为空则不过滤(全部同步) |

启用 LDAP 后，初始化管理员仍写入管理用户表（来源 LOCAL），通过 `HUB_AUTH__ADMIN_USERNAME` / `HUB_AUTH__ADMIN_PASSWORD` 与管理端登录端点登录。LDAP 同步用户的显示名取 `givenName`，缺失时回退到 `uid`。

如需使用 SQLite 本地开发，可直接运行后端二进制，并把 `web.dir` / `manager.dir` 指向前端构建产物目录；也可关闭前端端口，仅使用 API。拆分部署时使用同一镜像启动多个容器，通过 `web.enable` / `manager.enable` 控制各容器职责，不需要 nginx。


## License

MIT
