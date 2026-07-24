# Knot 智能能力闭环实施状态

> 本文件是跨 Agent 的状态交接入口。
> 开始任何阶段前，请先完整阅读 `ROADMAP.md`，再阅读本文件。

## 当前状态

- 当前阶段：阶段 4 已完成，等待进入阶段 5
- 最后完成阶段：阶段 4 - 上下文自动发现与 Evidence Manifest
- 当前 schema version：3
- 当前里程碑：个人年度总结闭环

## 已确认的代码基线

- 前端现有业务设置仍主要保存在 `src/services/settings.js` 的 `localStorage`；资料源注册表已改为后端 SQLite 持久化。
- Electron 的 `knot-settings.json` 当前只用于部分 Electron 设置。
- Go 后端使用纯 Go `modernc.org/sqlite v1.54.0`，数据库位于 `appdata.Resolve()` 目录下的 `knot.db`。
- `backend/api/archive_scan.go` 主要识别包含 `工作记录.md` 的目录。
- `backend/api/work_report.go` 由请求传入扫描路径。
- `backend/api/archive_ai_search.go` 由请求传入归档路径。
- `backend/policy` 与 `backend/safepath` 已统一本地/AI 策略和注册资料源相对路径解析；legacy 绝对路径仅保留在旧接口兼容层。
- `backend/indexer` 已提供可重建的有限增量索引；尚未建立 ContextManifest、Evidence 或任何新增 AI 调用。
- 工作区在创建本计划时已有用户修改：`README.md`。后续 Agent 不得覆盖或回退该修改。

## 阶段记录

## Stage 0 - 应用数据目录与基线固定

- 状态：completed
- 分支：`dev`
- 提交 SHA：`cc316f1`（实现前基线：`90dca300530945cc408dfe034ec7542cab13d0a9`）
- 完成日期：2026-07-23

### 已实现

- Electron 启动 Go 后端时，将 `app.getPath("userData")` 作为 `KNOT_DATA_DIR` 注入子进程环境，并保留其他父进程环境变量。
- 新增 `backend/appdata` 集中式数据目录解析包；后端启动时先完成解析，失败则拒绝启动。
- `KNOT_DATA_DIR` 可为独立启动的开发/测试后端显式指定绝对目录；未设置时回退到 `os.UserConfigDir()/Knot`。
- 相对路径覆盖会被拒绝，输入路径经 `filepath.Clean` 规范化。
- 后端日志只记录解析来源，不输出数据目录、凭据或用户文件内容。
- `.gitignore` 已覆盖仓库内 `/data/_test_*/` 测试运行目录。
- 已增加 Electron 环境注入测试、Go 数据目录覆盖/默认/错误测试和 Windows 路径规范化测试。

### 数据库与 migration

- schema version：无。
- 新增/变化：本阶段未引入 SQLite、数据库文件或 migration。

### 公共接口

- Go：`appdata.Resolve() (appdata.Directory, error)`。
- `appdata.Directory.Path`：清理后的绝对应用数据目录。
- `appdata.Directory.Source`：`environment` 或 `system_config`，供诊断使用。
- 环境变量：`KNOT_DATA_DIR`。Electron 正常启动时固定为 `app.getPath("userData")`；开发和测试直接运行后端时可显式覆盖。
- 默认行为：环境变量为空时使用系统用户配置目录的 `Knot` 子目录；解析函数本身不创建目录。

### 关键决策

- 数据目录解析集中在 `backend/appdata`，后续 SQLite、索引和审计不得自行推导应用数据路径。
- 显式覆盖必须是绝对路径，避免数据库位置随进程工作目录变化。
- 本阶段没有改变设置存储、扫描行为、安全策略或任何 AI 公共语义，因此不需要 ADR。

### 测试

- `pnpm test -- --run`：通过，5 个测试文件、15 个测试全部通过。
- `pnpm run build`：通过，Vite 生产构建完成；存在上游 CJS Node API 弃用警告。
- `$env:TEMP/TMP=<repo>/data/_test_appdata; conda run -n go go test ./...`：通过，`api`、`appdata`、`mail` 包通过。
- `$env:TEMP/TMP=<repo>/data/_test_appdata; conda run -n go go build -buildvcs=false ./...`：通过。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- 首次未覆盖 `TEMP/TMP` 的定向 Conda 测试未进入 Go 测试：当前 Windows 系统临时目录不可用；改用仓库内已忽略目录后通过。

### 已知问题

- 未启动 Electron 或 Go 前台监听服务做进程级验证；遵循有限测试要求，Electron 子进程环境由单元测试覆盖。
- 当前仅解析数据目录，不创建数据库或持久化数据；这些属于阶段 1。
- Vite 输出 CJS Node API 弃用警告，不影响本阶段测试与构建结果。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/appdata/appdata.go`、`electron/backendProcess.js`、`src/services/settings.js`、`src/hooks/useTaskDirectories.js`。
- 可复用接口：`appdata.Resolve()`、`appdata.EnvDataDir`、`appdata.Directory`。
- 不要改动：`KNOT_DATA_DIR` 的 Electron 注入语义、绝对路径要求、系统配置目录回退；不要让 storage/source 功能再次自行推导数据目录。

## Stage 1 - 后端资料源注册表

- 状态：completed
- 分支：`dev`
- 提交 SHA：未提交（实现前基线：`bfa10248c845f7cbcb088e6a68c636494f099bd0`）
- 完成日期：2026-07-23

### 已实现

- 新增后端 SQLite 生命周期与版本化 migration；后端启动时使用 `appdata.Resolve()` 的结果创建或打开 `knot.db`，migration 失败会拒绝启动。
- 新增 `source_roots` repository 和 registry，支持持久化 CRUD、输入枚举校验、缺失/离线/无权限可用性记录，以及读取时的可用性刷新。
- 资料源路径支持 `~` 当前用户目录展开、绝对路径要求、`filepath.Clean` 规范化和持久化 `path_key` 去重；Windows 下路径 key 忽略大小写并统一斜杠。
- 缺失目录仍可登记为 `availability=missing`，不会因为盘符、移动盘或网络目录当前不可用而阻止后端启动。
- legacy import 覆盖当前工作目录、扫描目录、部门归档和项目归档；部门/项目的 ID、名称和 scope 关系会保留，相同规范化路径只创建一次。
- React 启动时自动提交 legacy 资料源白名单；设置页提供“同步现有设置”、资料源列表、刷新、添加、编辑和删除，并提供专用“添加工作日志”入口。
- 删除资料源只删除注册记录，不删除任何目录或文件。
- 本阶段没有扫描目录、读取资料正文、创建全文索引、调用 AI 或修改旧工作报告/归档接口。

### 数据库与 migration

- schema version：1。
- 数据库：`<appdata.Resolve().Path>/knot.db`；文件名由 `backend/storage` 集中定义，调用方不得自行推导。
- migration：`backend/storage/migrations/001_source_roots.sql`。
- schema version 表：`schema_migrations(version, applied_at)`。
- 新增 `source_roots`：`id`、名称、类型、规范化路径与唯一 `path_key`、scope、启用/递归标记、本地/AI 访问级别、可用性及内部时间戳。
- 索引：`enabled + kind`、`scope_type + scope_id`；`path_key` 唯一。
- migration 在单一事务中执行；任何 SQL 或版本记录失败都会整体回滚，不留下部分 schema。
- 没有自动 down migration。需要恢复时应先退出 Knot，再恢复升级前的 `knot.db` 备份；数据库损坏且无备份时可移走数据库后重启并重新同步 legacy/手工资料源。此操作不会删除原始资料文件，但会丢失数据库内的手工注册记录。

### 公共接口

- Go：`storage.Open(ctx, appdata.Directory) (*sql.DB, error)`、`storage.CurrentSchemaVersion`。
- Go：`sources.NewRepository(db)`、`sources.NewRegistry(repository)`，registry 提供 `List/Get/Create/Update/Delete/ImportLegacy`。
- `GET /api/source-roots`：返回 `{ "source_roots": [...] }`。
- `POST /api/source-roots`：创建资料源，成功返回 `201`。
- `GET /api/source-roots/{id}`：读取单个资料源。
- `PUT /api/source-roots/{id}`：完整更新资料源。
- `DELETE /api/source-roots/{id}`：只删除注册记录，成功返回 `204`。
- `POST /api/source-roots/import-legacy`：幂等导入现有前端资料源设置。
- `SourceRoot` JSON：`id`、`name`、`kind`、`path`、`scope_type`、`scope_id`、`scope_name`、`enabled`、`recursive`、`local_access`、`ai_access`、`availability`；内部 `path_key` 和时间戳不对外暴露。

### 关键决策

- 按 ROADMAP 使用不依赖 CGO 的 `modernc.org/sqlite v1.54.0`；Windows 测试、Windows 项目打包编译和 Linux/amd64 `CGO_ENABLED=0` 交叉构建均通过，因此无需新增 ADR。
- `storage.Open` 只接受 `appdata.Resolve()` 产生的目录对象，延续阶段 0 的单一数据目录来源。
- 当前阶段只持久化 SourceRoot 声明值，不实现阶段 2 的有效策略优先级、敏感路径覆盖或安全相对路径解析。
- 可用性检测只执行目录级 `os.Stat`，不枚举文件，也不读取正文；列表和单项读取会刷新发生变化的状态。
- legacy import 冲突时保留已存在的资料源，不覆盖用户后来编辑的名称、权限或类型。
- 前端 legacy payload 只包含 `folderPath`、`scanPath` 及部门/项目的 `id/name/archivePath`；不会发送邮箱密码、AI Key、窗口或更新设置。
- 现有 `localStorage` 不删除：旧页面继续使用原设置，资料源注册表与现有工作流保持兼容。

### legacy 导入范围

- `folderPath` → `kind=current_work`、`scope_type=global`。
- `scanPath` → `kind=active_work`、`scope_type=global`。
- 部门 `archivePath` → `kind=work_archive`、`scope_type=department`，保留部门 ID/名称。
- 项目 `archivePath` → `kind=work_archive`、`scope_type=project`，保留项目 ID/名称。
- 未迁移：邮箱配置、窗口与标题设置、更新通道、AI 模型与密钥、命名规则、SOP、默认部门/项目、`useYearFolder` 等现有工作流设置。

### 测试

- `pnpm test -- --run`：通过，6 个测试文件、17 项测试。
- `pnpm run build`：通过；保留 Vite CJS Node API 弃用警告和既有大 chunk 警告。
- `$env:TEMP/TMP=<repo>/data/_test_stage1; conda run -n go go test ./...`：通过，`api`、`appdata`、`mail`、`sources`、`storage` 包通过。
- `$env:TEMP/TMP=<repo>/data/_test_stage1; conda run -n go go build -buildvcs=false ./...`：通过。
- `$env:TEMP/TMP=<repo>/data/_test_stage1; pnpm run backend:build:win`：通过，生成 Electron 使用的 Windows 后端可执行文件。
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false`：通过，临时产物位于已忽略的 `data/_test_stage1`。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- `git diff --check`：通过。

### 已知问题

- 当前 Windows 系统临时目录仍不可用；Conda/Go 验证需将 `TEMP/TMP` 指向已忽略的 `data/_test_stage1`。
- 未启动 Electron 或 Go 前台监听服务做进程级验证；REST 使用 `httptest`，前端使用单元测试和生产构建验证。
- Linux 验证为 Windows 上的 Linux/amd64 纯 Go 交叉构建，未在实际 UOS/Linux 设备运行。
- 可用性按创建、更新和读取时的 `os.Stat` 刷新，没有后台 watcher；后台扫描属于阶段 3。
- 旧工作报告、归档和资料检索接口仍按原契约接收路径；阶段 1 只建立注册表，不提前改造后续上下文或策略流程。
- Vite 仍报告 CJS Node API 弃用和既有大 chunk 警告。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/sources/`、`backend/storage/`、`backend/api/source_roots.go`、`backend/api/path_safety.go`、`backend/api/work_record.go`。
- 可复用接口：`sources.SourceRoot`、`sources.Registry`、`sources.NormalizePath`、`sources.DetectAvailability`、`api.Dependencies.SourceRegistry`。
- 数据库规则：不要修改 migration 001；任何 schema 变化新增连续 migration，并保持事务和 `schema_migrations` 版本记录。
- 阶段 2 应基于 `source_root_id + relative_path` 增加统一策略和安全路径解析；不要建立索引、读取新增正文或调用 AI。
- 不要改动：`KNOT_DATA_DIR`/`appdata.Resolve()` 语义、SourceRoot JSON 字段含义、legacy 路径接口兼容性；不要把本地访问和 AI 暴露重新混成一个布尔值。

## Stage 2 - 统一策略引擎与安全路径解析

- 状态：completed
- 分支：`dev`
- 提交 SHA：`fd414e8`（实现前基线：`766d6c4f82b035c625c8cc5d0e0d697b77831778`）
- 完成日期：2026-07-23

### 已实现

- 新增 `backend/policy` 统一策略包，集中定义并校验 `local_access=none|read|read_write`、`ai_access=none|metadata|content`；`sources` 继续保留原公共类型名和 JSON 契约，但枚举实现改为复用 `policy`。
- 有效策略将系统默认、资料源、项目、文件/工作记录和系统硬限制视为逐层收紧的上限；任何层级的更宽权限都不能覆盖已有 deny，`local_access=none` 也会阻止 AI 暴露。
- 敏感路径标记只产生 `ai_access=none`，不会自行压低本地权限；`NoAI`、`Private`、中文个人信息标记及原有照片项目 `ai_access: noai` 均保持 AI fail-closed。
- 新增 `backend/safepath`：以 `source_root_id + relative_path` 查询 `sources.Registry`，拒绝伪造/未知 root ID、绝对路径混入、卷相对路径、UNC、空字符和 `..` 越界。
- 安全解析同时比较词法路径和操作系统最终路径；允许安全的缺失末级路径，但会解析所有已存在父级。Windows 使用 `GetFinalPathNameByHandle` 展开符号链接和 junction，越出 root 返回 `symlink_escape`。
- 提供受限的 `ResolveRegisteredAbsolute` 兼容桥：旧接口的敏感绝对路径只有能反向映射到已登记资料源并再次通过 ID 解析时才可本地处理。
- 生产路由中的归档扫描/列表允许读取已登记、在线且本地可读的敏感资料源；工作记录更新、归档、批量归档和恢复还要求该敏感资料源为 `local_access=read_write`。未登记敏感路径继续拒绝或跳过。
- 旧日报、周报、综合工作报告和资料检索仍保留原请求契约；AI 内容入口改为复用统一策略，`metadata` 工作记录不会作为正文候选，敏感路径仍在 AI 调用前拒绝。
- 没有创建索引、读取新的资料类型、增加 AI 调用、增加文件操作类型或改变现有 REST 请求字段。

### 数据库与 migration

- schema version：1。
- 新增/变化：无数据库结构变化，无新增 migration；`source_roots.local_access` 与 `source_roots.ai_access` 的既有字段和约束保持不变。

### 公共接口

- Go：`policy.Evaluate(policy.Evaluation) (policy.Decision, error)`；支持 defaults/source/project/document/system 的严格合并。
- Go：`policy.Decision` 提供 `AllowsLocalRead`、`AllowsLocalWrite`、`AllowsAIMetadata`、`AllowsAIContent` 和 `HasReason`。
- Go：`policy.HasSensitivePathMarker`、`policy.ParseLegacyAIAccess`。
- Go：`safepath.NewResolver(safepath.SourceRootRegistry)`。
- Go：`(*safepath.Resolver).Resolve(ctx, safepath.Request{SourceRootID, RelativePath})`。成功返回规范化相对路径、内部绝对/最终路径、Windows 不区分大小写的 path key 及有效策略；绝对路径字段不参与 JSON 输出。
- Go：`(*safepath.Resolver).ResolveRegisteredAbsolute` 仅用于尚未迁移完的 legacy 绝对路径兼容，不应成为阶段 3 新代码的主入口。
- 错误：`safepath.Reason(err)` 返回结构化 reason code。
- REST：没有新增 endpoint，也没有改变 SourceRoot JSON；`POST/PUT /api/source-roots` 对非法访问枚举继续返回 `400`。

### reason code

- 根与访问：`root_disabled`、`root_offline`、`local_access_denied`、`ai_access_denied`、`metadata_only`。
- 路径：`sensitive_path_marker`、`path_outside_root`、`symlink_escape`。
- 为后续扫描/候选过滤预留：`unsupported_type`、`size_limit`。

### 关键决策

- 严格度顺序为 `none < read < read_write` 和 `none < metadata < content`；各层取更严格值，项目或文件层不能放宽资料源层，系统层和 root 状态最后收紧。
- 敏感路径同时返回 `sensitive_path_marker` 与 `ai_access_denied`，便于后续 ContextManifest 对用户解释；`metadata` 返回 `metadata_only`，阶段 3/4 必须据此省略正文。
- 已登记 root 为 disabled、非 online 或 `local_access=none` 时，安全解析拒绝本地读取；离线、缺失、权限不足和 unknown 当前统一归入 `root_offline`。
- 旧普通绝对路径接口为兼容现有工作流暂不强制要求资料源 ID；只有原本被敏感标记拒绝的路径新增“已登记后可本地处理”例外。阶段 3 新代码不得继续扩散绝对路径入口。
- Windows junction 不能只依赖 `filepath.EvalSymlinks`；本机测试证实其未展开 junction，因此使用句柄级最终路径校验。该实现不改变公共契约，无需 ADR。

### 测试

- `pnpm test -- --run`：通过，6 个测试文件、17 项测试。
- `pnpm run build`：通过；保留 Vite CJS Node API 弃用警告和既有大 chunk 警告。
- `$env:TEMP/TMP=<repo>/data/_test_stage2; conda run -n go go test ./...`：通过，新增 `policy`、`safepath` 及既有 Go 包全部通过。
- `conda run -n go go test -v ./safepath`：通过；Windows junction 越界用例实际执行并返回 `symlink_escape`，未跳过。
- `$env:TEMP/TMP=<repo>/data/_test_stage2; conda run -n go go build -buildvcs=false ./...`：通过。
- `$env:TEMP/TMP=<repo>/data/_test_stage2; pnpm run backend:build:win`：通过。
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false`：通过，产物写入已忽略的 `data/_test_stage2`。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- 本阶段修改 Go 文件的 `gofmt -l` 与 `git diff --check`：通过。

### 已知问题

- 路径校验与后续文件打开/移动之间仍存在文件系统 TOCTOU 窗口；旧接口每次请求都会重新校验，但无法阻止其他进程在校验后替换链接。阶段 8/9 的执行器必须在操作前再次校验并结合文件状态/hash。
- 普通 legacy 绝对路径为兼容旧工作流仍可不经资料源注册表；阶段 3 的索引入口必须只接受 root ID 与相对路径，不能复用此兼容豁免。
- Linux 仅完成 amd64、`CGO_ENABLED=0` 交叉构建；Unix symlink 逻辑使用 `filepath.EvalSymlinks`，尚未在真实 UOS/Linux 运行。
- 未启动 Electron 或 Go 前台服务做进程级验证；REST 使用 `httptest`，路径与策略使用单元测试。
- 当前 Windows 系统临时目录仍不可用；Conda/Go 验证继续使用已忽略的 `data/_test_stage2`。
- 仓库全量 `gofmt -l .` 仍列出本阶段未修改的既有文件 `backend/mail/client_test.go`；本阶段修改的 Go 文件均已格式化。
- 本阶段只给出 `metadata_only` 策略结论；不会创建 evidence。正文省略属于阶段 3/4 消费策略时的责任。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/policy/`、`backend/safepath/`、`backend/sources/`、`backend/api/work_record.go`、`backend/api/archive_scan.go`。
- 可复用接口：`sources.Registry`、`safepath.Resolver.Resolve`、`policy.Evaluate`、`policy.Decision`、全部 reason code。
- 数据库：阶段 3 如建立 `indexed_documents`，新增连续 migration `002` 并将 schema version 更新为 2；不要修改 migration 001。
- 扫描规则：每个候选必须从 root ID 和相对路径重新解析；`ai_access=none` 只保存允许的最小本地元数据，`metadata` 不保存/生成正文片段，链接越界必须记录结构化排除原因。
- 不要改动：枚举含义、严格合并规则、敏感标记 AI deny、Windows 最终路径校验、legacy API 的兼容范围；不要建立第二套路径标记或权限判断。
- 不要提前实施：ContextManifest、Evidence、AI 调用、DOCX/PDF 正文、embedding、后台 watcher 或文件操作计划。

## Stage 3 - 本地增量索引与首批资料适配器

- 状态：completed
- 分支：`dev`
- 提交 SHA：`52aa312`（实现前基线：`fd414e82e5644fe6281812c4d125a7518feb2fe4`）
- 完成日期：2026-07-23

### 已实现

- 新增 `backend/indexer` 本地索引包，统一保存工作记录、日报、周报、普通 Markdown/TXT 和其他文件的可重建 `IndexedDocument`。
- 每个目录和文件候选都以 `source_root_id + relative_path` 重新调用 `safepath.Resolver.Resolve`；扫描 API 不接受 legacy 绝对路径，链接越界会返回结构化 `symlink_escape`，不会读取目标正文。
- 新增 source adapter 接口：`Name`、`Version`、`Matches`、`ReadsContentForMetadata`、`Extract`。生产适配器顺序为工作记录、Markdown、TXT、通用文件元数据。
- 工作记录适配器通过 API 层注入现有 `readWorkRecord`，复用既有 frontmatter、标题、日期、核心内容和 `ai_access` 解析；没有复制第二套工作记录解析器。
- Markdown extractor 识别文件名、frontmatter 和一级标题中的日报/周报类型，支持常见日期、`YYYY.Wnn` 和中文周次；仅日报、周报及工作记录保存上限内的正文片段，普通 Markdown/TXT 和其他文件不保存正文片段。
- 增量判定使用资料源 ID、规范化相对路径、文件大小、修改时间、adapter version 和内容 SHA-256。大小/时间/版本/策略均未变化时直接 touch，不重新读取或解析；`force=true` 可强制重建。
- 文件移动会产生一个新文档并将旧路径标为 `deleted`；删除文件保留文档 ID 和元数据、清空正文片段；离线/失效资料源将现有非 deleted 文档标为 `stale`，恢复扫描后重新变为 `ready`。
- `ai_access=none` 和敏感路径只保存文件名、相对路径、类型、大小、修改时间及内容 hash 等本地索引元数据，不解析或保存正文片段；`metadata` 不保存正文片段。工作记录显式 `noai`/非法权限值继续 fail-closed。
- 默认忽略：`.git`、`node_modules`、`.venv`、`venv`、`__pycache__`、`dist`、`build`、`target`、`.cache`、`coverage`、`.next`、`.nuxt`、`.output`、`.vite`、`.turbo`、`.pytest_cache`、`.mypy_cache`、`.ruff_cache`。
- 扫描为同步有限任务，复用 HTTP request context 支持取消，不启动 watcher。达到文件数、读取字节或时间上限时返回 `partial`，且不会把本轮未看到的旧文档误标为 deleted。
- 没有调用 AI、建立 ContextManifest/Evidence、解析 DOCX/PDF 正文、创建 embedding、增加后台监听或实现文件操作。

### 数据库与 migration

- schema version：2。
- migration：`backend/storage/migrations/002_indexed_documents.sql`；未修改 migration 001。
- 新增 `indexed_documents`：稳定文档 ID、资料源 ID、相对路径及 Windows 规范化 key、文档类型、标题、任务日期、项目 ID、大小/修改时间/hash、受限片段、索引状态、有效 AI 权限、工作记录权限覆盖、策略原因、adapter/version、错误码和扫描代次。
- 状态枚举：`ready | stale | deleted | error`；AI 权限继续复用 `none | metadata | content`。
- 唯一约束：`source_root_id + relative_path_key`；索引覆盖资料源/状态、文档类型/日期、项目/日期。
- `source_root_id` 使用 `ON DELETE CASCADE`：删除资料源注册记录会同步删除其可重建索引行，但不会删除原始文件；既有 SourceRoot DELETE API 语义保持不变。
- migration 仍由 `storage.Migrate` 在单一事务中连续执行并记录 `schema_migrations`。没有自动 down migration；恢复 schema 1 需退出 Knot 后恢复升级前数据库备份。
- 索引重建无需删除数据库：对单一资料源或全部启用资料源调用 scan API 并传入 `{ "force": true }`。

### 公共接口

- Go：`indexer.NewRepository(db)`；repository 提供按资料源/路径读取、保存/touch、缺失标记 deleted 和资料源标记 stale。
- Go：`indexer.NewScanner(registry, resolver, repository, adapters...)`。
- Go：`indexer.DefaultLimits()` 当前为最多 10,000 个文件、512 MiB 实际读取量、单文件 10 MiB、30 秒、单片段 1,200 rune、最多返回 100 条问题。
- Go：`indexer.Adapter`、`indexer.Extraction`、`indexer.IndexedDocument`、`indexer.ScanOptions`、`indexer.ScanResult`、`indexer.ScanAllResult`。
- `POST /api/source-roots/{id}/scan`：扫描单一登记资料源；请求只支持可选 `{ "force": boolean }`。
- `POST /api/source-roots/scan`：在同一全局文件数/字节/耗时预算内扫描全部 enabled 资料源；离线资料源单独返回 `unavailable`，不会导致其他可用资料源索引被删除。
- 两个 API 都不接受路径；未知/伪造 root ID 返回 `404`，索引服务未初始化返回 `503`。

### 支持范围与字段

- `工作记录.md` → `work_record`：标题、任务日期、项目 scope、内容 hash、受策略保护的有限核心片段、工作记录 `ai_access` 覆盖。
- 日报 Markdown → `journal_daily`：标题、日期、hash、有限片段。
- 周报 Markdown → `journal_weekly`：标题、周起始日期、hash、有限片段。
- 普通 Markdown → `markdown`；TXT → `text`：标题/文件名、日期（可从安全文件名识别）、大小、修改时间、hash，不保存正文片段。
- 其他文件 → `file`：文件名、大小、修改时间、hash；超过单文件上限时仍保存最小元数据并记录 `size_limit`，不读取或解析内容。
- PDF/DOCX 当前只会落入通用 `file` 元数据适配器，不解析正文。

### 关键决策

- 阶段 3 新入口完全基于资源 ID 和相对路径，没有复用 `ResolveRegisteredAbsolute`；旧接口兼容范围未扩大。
- hash 是本地、不可逆的变更指纹，不是正文片段，也不会改变 `ai_access_effective`；阶段 4 仍必须重新应用当前策略，不能把 hash 或 stale/deleted/error 文档视为可发送证据。
- 未完成/取消/到达上限的扫描不执行缺失删除，避免部分遍历破坏已有索引状态。
- 普通 Markdown 只为识别日志类型和标题做本地有限解析；只有明确日报/周报会保存受限片段。
- 当前契约与 ROADMAP 一致，无需 ADR，也没有新增 Go module 依赖。

### 测试

- `pnpm test -- --run`：通过，6 个测试文件、17 项测试。
- `pnpm run build`：通过；保留 Vite CJS Node API 弃用警告和既有大 chunk 警告。
- `$env:TEMP/TMP=<repo>/data/_test_stage3_runtime; conda run -n go go test ./...`：通过，全部 Go 包通过。
- `conda run -n go go test -v ./indexer`：8 项通过；Windows 链接越界用例实际执行并通过，未跳过。
- `conda run -n go go vet ./...`：通过。
- `conda run -n go go build -buildvcs=false ./...`：通过。
- `pnpm run backend:build:win`：通过。
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false`：通过，临时产物写入已忽略目录。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- `go mod tidy -diff`：无差异。
- 本阶段修改 Go 文件的 `gofmt -l`：无输出；`git diff --check`：通过。

### 已知问题

- 增量快路径假设相同相对路径、大小和修改时间代表未变化；外部工具若在修改内容后刻意恢复相同 mtime 和大小，需要使用 `force=true` 重建才能重新计算 hash。
- 取消和 30 秒上限在目录项之间检查；已经进入的单次操作系统文件读取不能被 Go context 中途抢占，但单文件 10 MiB 上限限制了正常读取规模。网络盘底层 I/O 长时间阻塞仍需阶段 9 做更强隔离。
- 路径解析到实际读取之间仍存在阶段 2 已记录的 TOCTOU 窗口；扫描会逐候选重新解析，但不能阻止其他进程在校验后替换文件/链接。
- 离线资料源会保留此前允许保存的有限片段并标为 stale；阶段 4 必须只使用 ready 文档并重新校验当前策略。资料源恢复或权限变化后的扫描会重建/清除不再允许的片段。
- 当前扫描 API 为同步请求，没有索引状态 UI、持久 scan job 或后台 watcher；这些不是阶段 3 必需项。
- Linux 仍只完成 amd64、`CGO_ENABLED=0` 交叉构建，未在真实 UOS/Linux 验证 symlink 与网络盘行为。
- 仓库既有 `backend/mail/client_test.go` 仍会被全量 `gofmt -l .` 列出，本阶段未修改。
- Vite 仍报告 CJS API 和大 chunk 警告。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/indexer/`、`backend/extractors/markdown.go`、`backend/storage/migrations/002_indexed_documents.sql`、`backend/policy/`、`backend/safepath/`、`backend/sources/`。
- 可复用接口：`indexer.Repository.ListBySource`、`indexer.IndexedDocument`、`indexer.Scanner`、`policy.Decision`、`safepath.Resolver.Resolve`、全部 index/policy reason code。
- 阶段 4 应只查询 `ready` 文档，按当前 SourceRoot 和相对路径重新解析/判权，再形成 ContextManifest/Evidence；不要信任索引中缓存的权限结论、stale 状态或 deleted/error 文档。
- 阶段 4 discover 请求不得接收路径，不得调用 AI；对 `ai_access=none` 完全排除，对 `metadata` 不生成正文 evidence，并把离线/错误/策略原因转为可解释缺口。
- 不要改动：migration 001/002、SourceRoot/权限枚举含义、严格策略优先级、敏感路径 AI deny、Windows 最终路径校验、legacy API 兼容范围、adapter 的有限读取边界。
- 不要提前实施：年度总结生成、AI Gateway、DOCX/PDF 正文、embedding、通用 Tool Calls、watcher 或文件操作计划。

## Stage 4 - 上下文自动发现与 Evidence Manifest

- 状态：completed
- 分支：`dev`
- 提交 SHA：`306dacb`（实现前基线：`18471be`）
- 完成日期：2026-07-24

### 已实现

- 新增 `backend/contextmanifest`，首个 task profile 固定为 `personal_annual_summary`；discover 请求只接受任务类型、查询文本和明确的起止日期，不接受任何扫描路径或归档路径。
- Context Resolver 只查询 `journal`、`current_work`、`active_work`、`work_archive` 四类登记资料源；停用资料源进入 `excluded`，离线/缺失/无本地读取权限的资料源进入 `unavailable_sources`，不会阻断其他资料源发现。
- 候选只使用 `ready` 索引文档。每个候选都会重新调用 `safepath.Resolver.Resolve`，比较当前文件大小/mtime，并把工作记录显式权限覆盖与当前资料源/敏感路径策略重新合并；不信任索引缓存的 `ai_access_effective`。
- `ai_access=none` 文档只进入本地排除说明；`ai_access=metadata` 可形成无正文的 EvidenceItem；`ai_access=content` 只使用阶段 3 已保存的受限片段，阶段 4 再裁剪至最多 800 rune。普通 Markdown/TXT/通用文件仍然只有元数据，不读取 DOCX/PDF 正文。
- 候选排序稳定且可解释：先要求时间范围命中，再累计任务日期、资料类型、标题/项目关联、完成/推进/成果关键词和近期活动分数；最终按分数降序、日期降序、root ID、相对路径和 document ID 打破平局。
- Evidence ID 根据 document ID、hash、mtime、当前有效 AI 权限和受限片段确定性生成；相同请求与相同索引产生相同 evidence 顺序和 ID。每次 discover 仍创建新的持久 manifest ID。
- token 估算对 ASCII 使用约 4 字符/token、非 ASCII 使用约 1 rune/token，并加入请求/evidence 结构开销；默认预算 12,000、最多 100 条 evidence。低排序候选超过数量或预算时以 `token_budget` 进入排除说明。
- discover 默认最多检查 20,000 个索引文档、返回 200 条排除明细（同时保留完整排除总数）、运行 30 秒；Evidence 正文最多 800 rune。所有限制均在本地执行。
- manifest 默认有效 30 分钟并持久化所选文档快照。`GET` 会重新检查 TTL、索引状态/hash、文件大小/mtime、安全路径和当前策略；任何变化都会返回 `valid=false`、`requires_rediscovery=true` 和结构化失效原因。
- 本阶段没有新增 AI 调用、生成年度总结、解析 DOCX/PDF 正文、embedding、通用工具循环、文件修改或前端个人总结页面。

### 数据库与 migration

- schema version：3。
- migration：`backend/storage/migrations/003_context_manifests.sql`；未修改 migration 001/002。
- 新增 `context_manifests`：保存 task type、query、明确时间范围、完整 manifest JSON、快照 hash、创建时间和过期时间。
- 新增 `context_manifest_documents`：保存入选 document ID、root ID、相对路径、content hash、mtime、大小和发现时有效 AI 权限，用于读取 manifest 时重新校验。
- 文档快照不对 `indexed_documents` 建外键：资料源/可重建索引被删除后 manifest 仍可读取并明确返回 `document_missing`，不会静默丢失失效依据。
- 没有自动 down migration；恢复 schema 2 需退出 Knot 后恢复升级前数据库备份。删除 SQLite 仍可通过重新登记/扫描恢复原始文件索引，但历史 manifest 会丢失。

### 公共接口

- `POST /api/context/discover`：成功返回 `201`；请求契约为：

```json
{
  "task_type": "personal_annual_summary",
  "query": "根据过去一年工作资料生成约 1500 字个人工作总结大纲",
  "period_start": "2025-07-24",
  "period_end": "2026-07-24"
}
```

- `GET /api/context/{id}`：读取持久 manifest 并执行当前有效性校验；未知 ID 返回 `404`，服务未初始化返回 `503`。
- `ContextManifest` 公开字段包含：ID、任务/时间、无绝对路径的资料源摘要、EvidenceItem、排除明细与计数、不可用资料源、token 估算/预算、确认要求、有效性/重新发现状态和创建/过期时间。
- `EvidenceItem` 公开字段包含：evidence/document/root ID、source type、标题、日期、项目名、受限 excerpt、入选 reason 和当前有效 AI 权限；metadata evidence 的 `excerpt` 固定为空字符串。
- `indexer.Repository.GetByID` 与 `IndexedDocument.DocumentAIAccess()` 是阶段 4 新增的只读复用接口，分别用于 manifest 失效校验和重新应用文档级权限覆盖。

### 示例 discover 响应

```json
{
  "id": "context_xxx",
  "task_type": "personal_annual_summary",
  "period_start": "2025-07-24",
  "period_end": "2026-07-24",
  "sources": [
    {
      "source_root_id": "root_journal",
      "name": "工作日志",
      "kind": "journal",
      "candidate_documents": 12,
      "evidence_items": 8
    }
  ],
  "evidence": [
    {
      "id": "evidence_xxx",
      "document_id": "doc_xxx",
      "source_root_id": "root_journal",
      "source_type": "journal_weekly",
      "title": "2026.W03 工作周报",
      "date": "2026-01-12",
      "project": "",
      "excerpt": "完成……推进……",
      "reason": "时间范围匹配；使用文档任务日期；周报优先；包含完成、推进或成果表述；采用受限正文片段",
      "ai_access_effective": "content"
    }
  ],
  "excluded_count": 3,
  "unavailable_sources": [],
  "estimated_input_tokens": 1680,
  "token_budget": 12000,
  "requires_confirmation": true,
  "status": "ready",
  "valid": true,
  "requires_rediscovery": false
}
```

示例省略了部分始终存在的空数组、scope 和时间字段；生产响应不会包含 SourceRoot 绝对路径。

### 关键决策

- manifest 只保存入选文档快照；排除候选或随后新增文档不会改变既有 manifest，TTL 到期后通过重新 discover 刷新范围。入选文档的 hash/元数据/策略变化会立即使 manifest 失效。
- discover 不自动触发 scan，避免一次预览请求隐式递归文件系统；索引不存在或文件元数据比索引新时返回零候选或 `index_outdated`，由调用方提示用户扫描后重试。
- 当前没有项目级独立策略表；有效策略严格复用阶段 2 已有的资料源、敏感路径和工作记录显式覆盖语义。未新增第二套路径标记或权限枚举。
- API 对未知 JSON 字段 fail closed，因此 `scan_paths`、`archive_paths`、绝对路径或未来未声明字段都会返回 `400`。
- 契约与 ROADMAP 的安全边界和公共语义一致，无需 ADR。

### 测试

- `pnpm test -- --run`：通过，6 个测试文件、17 项测试。
- `pnpm run build`：通过；保留 Vite CJS Node API 弃用警告和既有大 chunk 警告。
- `$env:TEMP/TMP=<repo>/data/_test_stage4_runtime; conda run -n go go test ./...`：通过，全部 Go 包通过。
- `conda run -n go go test ./contextmanifest ./storage ./api`：通过；覆盖稳定排序、NoAI、metadata、离线缺口、预算、索引过期、hash/策略/TTL 失效、API 无路径契约和 v2 → v3 migration。
- `conda run -n go go vet ./...`：通过。
- `conda run -n go go build -buildvcs=false ./...`：通过。
- `pnpm run backend:build:win`：通过。
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false`：通过，临时产物写入已忽略目录。
- `node --check electron/main.js`：通过。
- `node --check electron/preload.js`：通过。
- `node --check electron/backendProcess.js`：通过。
- `go mod tidy -diff`：无差异。
- 本阶段修改 Go 文件的 `gofmt -l`：无输出；`git diff --check`：通过。

### 已知问题

- discover 依赖最近一次有限扫描；文件若在扫描后修改但大小/mtime 不同，会返回 `index_outdated`，不会使用旧片段。沿用阶段 3 的极端风险：内容变化后又恢复相同大小和 mtime 时，需要 `force=true` 扫描才能得到新 hash。
- manifest 有效性只跟踪入选 Evidence 文档。排除项变化或新增相关文档不会在 30 分钟 TTL 内主动使现有 manifest 失效；重新 discover 可立即刷新候选范围。
- 排序是确定性的本地关键词启发式，不做分词、语义搜索或 embedding；中文/英文完成表述之外的同义表达可能排序较低。
- 普通 Markdown/TXT、DOCX/PDF 和其他文件在阶段 3 只有元数据，因此阶段 4 即使允许 content 也不会虚构 excerpt。DOCX/PDF 正文属于阶段 6。
- 路径解析与随后 `os.Stat` 之间仍存在 TOCTOU 窗口；阶段 5 生成前必须再次调用 manifest 校验，阶段 9 继续加强文件系统竞态防护。
- `excluded` 明细默认最多保存 200 条，但 `excluded_count` 和 `omitted_excluded_count` 保留总量信息；候选检查最多 20,000 个文档，达到上限要求缩小资料源或时间范围后重新发现。
- 当前没有阶段 5 的个人总结 UI，也没有 evidence 勾选/取消接口；本阶段只提供后端 discover/get 契约。
- Linux 仍只完成 amd64、`CGO_ENABLED=0` 交叉构建，未在真实 UOS/Linux 验证。

### 下一阶段入口

- 必须先阅读：`ROADMAP.md`、本文件、`backend/contextmanifest/`、`backend/api/context_manifest.go`、`backend/storage/migrations/003_context_manifests.sql`、`backend/indexer/`、`backend/policy/`、`backend/safepath/`。
- 可复用接口：`contextmanifest.Resolver.Discover`、`contextmanifest.Resolver.Get`、`contextmanifest.ContextManifest`、`contextmanifest.EvidenceItem`、`contextmanifest.Repository`、`indexer.Repository.GetByID`。
- 阶段 5 应新增独立“个人总结”页面：先调用 discover 展示资料源、排除/离线缺口和 token 预算，再让用户确认/取消 evidence，最后只按 manifest ID 生成。
- 阶段 5 的 generate 入口必须在 AI 调用前重新调用 manifest 校验并只发送仍被允许、仍在用户确认集合中的 EvidenceItem；`valid=false` 或 `requires_rediscovery=true` 必须拒绝生成。
- AI 输入不得包含 `excluded`、`unavailable_sources`、SourceRoot 绝对路径、API Key 或数据库内部快照；metadata evidence 不得补读正文。
- 不要改动：discover 的无路径请求、明确时间范围、严格未知字段校验、默认资料源种类、30 分钟失效和当前路径/策略/hash 重校验语义。
- 不要提前实施：完整正文生成、DOCX/PDF 抽取、embedding、通用 Tool Calls、文件修改或 action plan。

## 全局未决事项

- 阶段 5 需要确定用户取消 evidence 的请求契约、AI run/audit schema、结构化大纲输出和 evidence 引用校验。
- 阶段 5 已确定新增独立“个人总结”页面，并复用现有工作报告基础组件；不要把新闭环塞入现有 `WorkReport` 状态机。
- 阶段 6 需要分别为 DOCX 和 PDF 解析依赖记录 ADR。

## 固定验证命令

```powershell
pnpm test -- --run
pnpm run build
conda run -n go go test ./...
conda run -n go go build -buildvcs=false ./...
node --check electron/main.js
node --check electron/preload.js
```
