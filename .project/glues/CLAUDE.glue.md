<README.md>
@README.md
</README.md>

## 目录说明
### 后端服务(hub-server/)
模块描述: 系统后端服务功能实现,提供api给cli,web,manager实现业务功能。
模块类型: 后端服务。
技术栈：go1.26, gorm, 数据库:mysql8|sqlite|postgresql, 部署:docker, 打印日志:slog, 命令行参数:cobra
部署: 编译docker镜像, 容器中启动.
技术要求:
- 修改数据库先修改数据库设计，gorm自动变更数据库结构。
- 需要打印debug日志，便于调试。
编译环境:
- GOPROXY="https://goproxy.cn,direct"

> 详细要求参考docs/design/app-server.md

### 提示词工具库(prompts-sdk/)
模块描述: 提示词表达式解析、执行,提供给server和cli使用。
模块类型: go库
技术栈: go1.26 打印日志:slog
编译环境:
- GOPROXY="https://goproxy.cn,direct"

> 详细要求参考docs/design/sdk.md

### 命令行客户端(cli/)
模块描述: hub命令行客户端
模块类型: 命令行工具,类似命令：npm
技术栈: go1.26 打印日志:slog, 命令行参数:cobra
编译结果: linux,mac,windows
CICD: 代码提交，github workflow自动编译不同环境的版本。
编译环境:
- GOPROXY="https://goproxy.cn,direct"

> 详细要求参考docs/design/app-cli.md

### 用户端门户(web/)
模块描述: 用户端界面，用户搜索、浏览、查看、制作提示词的界面
模块类型: 前端项目

{{dev-frontend-reactjs-stack}}

{{dev-frontend-rule-react}}

> 详细要求参考docs/design/app-web.md

### 管理界面(manager/)
模块描述: 系统管理界面, 运营管理功能。
模块类型: 前端项目

{{dev-frontend-reactjs-stack}}

{{dev-frontend-rule-react}}

> 详细要求参考docs/design/app-manager.md

### 项目文档(docs/)
- `design/`: 设计文档，README.md(设计总体说明);dicts.md(数据字典全局定义);prompts.md(提示词制作规则);sdk.md(提示词处理sdk);api.md(api总体设计说明);db.md(数据库设计总体说明);app.md(应用总体设计说明);模块设计文件名:{db|app|api|open|ui...}-{模块名}.md, db-数据库设计, app-应用功能设计, ui-文本ui设计(文本线框图);
- `guides/`: 系统使用手册
- `test/`: 测试相关文档，cases:案例, logs:测试记录, reports:测试记录

### 项目过程记录(.project/)
项目过程记录目录

#### 项目过程记录(.project/plans)
- 项目计划, 文件命名规范:{app|模块名}-计划名.md
- `done/`: 完成的计划放到这个目录下

#### 项目过程记录(.project/issues)
- 项目需求和问题, 文件命名规范:{bug|feature|refactor|improve|doc}-{app|模块名}-{issue name}.md
- `fixed/`: 解决了的issue，移动到`fixed`目录下
- `closed/`: 确认完成的issue, 移动到`closed`目录下

{{template(dev-backend-config){应用同时支持配置文件、环境变量、命令行参数配置,环境变量前缀:HUB_}}}

{{doc-design-rule}}

{{dev-project-rule-base}}