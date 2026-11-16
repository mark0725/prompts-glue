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

**前端技术栈**
Reactjs,TypeScript,TailwindCSS,Vite,UI Components(shadcn-ui),icons(lucide-react),Zustand,Recharts,i18next

**前端技术要求**
- 前端所有用到的编码统一通过后端数字典api加载，数据字典由后端统一管理，有的支持动态可配置，禁止在前端代码中硬编码。
- 禁止使用浏览器原生对话框组件，使用react组件。

> 详细要求参考docs/design/app-web.md

### 管理界面(manager/)
模块描述: 系统管理界面, 运营管理功能。
模块类型: 前端项目

**前端技术栈**
Reactjs,TypeScript,TailwindCSS,Vite,UI Components(shadcn-ui),icons(lucide-react),Zustand,Recharts,i18next

**前端技术要求**
- 前端所有用到的编码统一通过后端数字典api加载，数据字典由后端统一管理，有的支持动态可配置，禁止在前端代码中硬编码。
- 禁止使用浏览器原生对话框组件，使用react组件。

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

应用配置模板

- 支持配置方式：配置文件、环境变量、命令行参数
- 环境变量前缀：HUB_
- 配置文件格式：YAML、JSON、TOML
- 配置优先级（从高到低）：命令行参数 > 环境变量 > 配置文件 > 代码默认值

**环境变量命名规则**
- 格式：`{APPNAME}__` 开头（注意：此处未提供 APPNAME，应使用占位或标注未提供。但用户指定前缀为HUB_，故APPNAME应为HUB？用户未明确APPNAME，仅给出前缀。按照输入原文：“以`{APPNAME}_`开头”，但额外要求环境变量前缀:HUB_，推测APPNAME就是HUB。但为避免虚构，保留为`{APPNAME}`占位，并说明实际前缀为HUB_？但用户要求“不得虚构内容”，所以保留`{APPNAME}`占位，标注未提供具体名称。）
  实际上用户输入原文中使用了`{APPNAME}_`，但在额外要求中指定了环境变量前缀:HUB_。这可能意味着APPNAME就是HUB。为准确，我认为可以写成`HUB_`代替`{APPNAME}_`。因为用户明确说“环境变量前缀:HUB_”，所以这里不再使用`{APPNAME}`占位。我决定用HUB_。
- 双下划线`__`表示层级中的点号。
- 示例：`HUB_LLM__MODEL` 对应 `llm.model`
- 数组类型：使用`__数组元素位置__`，如`__0__`、`__1__`

**命令行参数规则**
- 格式：`--key` 或 `--object-key {值}` 或 `--object-key=值`
- 横杠`-`表示层级中的点号。
- 示例：`--llm-model deepseek-flash` 等价于设置 `llm.model=deepseek-flash`
- 数组配置：使用多个相同参数选项（如 `--array val1 --array val2`）依次指定元素。

## 设计注意事项(docs/design)
- 设计模块结构清晰，层次、职责、关系分明，避免多模块实现相似功能，避免类型的多处重复定义。
- 用自然语言或图形描述设计(mermaid|svg|drawio|plantuml等文本绘图语言)，不应出现代码实现说明，尽量使用图形和表格表达设计。
- README.md(设计总体说明);dicts.md(数据字典全局定义);api.md(api总体设计说明);db.md(数据库设计总体说明);app.md(应用总体设计说明);模块设计文件名:{db|app|api|open|ui}-{模块名}.md, db-数据库设计, app-应用功能设计, ui-文本ui设计(文本线框图);
- 调整设计(`docs/design/`)子模块设计时，同步更新相关全局设计文档`docs/design/{db|api|app...}.md`和`docs/design/README.md`

## 注意事项
- 这是用于实际生产的代码，用工程化的方法设计和开发，不要用写demo的方式(硬编码测试数据、硬编码参数等临时方法)实现代码，要考虑正确性、可维护性、可靠性、安全性、性能、容错性、易用性、美观性、可扩展性、适应性、灵活性、兼容性、交互体验等工程项目基本特性要求。
- 根据设计实现代码，根据用户需求->分析现有设计和实现->调整设计->设计review->实现代码->验证代码->代码review。设计描述应该简洁、明了，拒绝冗繁。
- 不确定的部分不要随意推断和创造，提示用户补充必须的信息。
- 发现提示词或设计文档中有逻辑矛盾的部分，应该提示用户，经用户确认后更新提示词或文档后再调整实现。
- 项目全局README.md内容:项目介绍、系统安装、配置相关方法，不要将系统设计相关内容放到此文件，设计内容应该放到`docs/design`目录的对应设计文档中。项目概要、安装、配置发生变化时，需要同步修改项目的README.md